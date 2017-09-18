// Copyright 2015 zxfishhack
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

/*
	#cgo CFLAGS: -I../src -I../include -D_CGO_BUILD_
	#include <stdlib.h>
	#include <string.h>
	#include <stdint.h>
	#include "easyraft_inernal.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/coreos/etcd/pkg/fileutil"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/snap"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/pkg/capnslog"
)

type raftServer struct {
	inter *raftNodeInternal
	snap  *snap.Snapshotter
	ctx   unsafe.Pointer
	node  *raftNode
	wgMu  sync.RWMutex
	wg    sync.WaitGroup
	cfg   config
	stopc chan struct{}

	// for debug
	attachCount uint64
	doneCount   uint64
}

var (
	plog = capnslog.NewPackageLogger("github.com/zxfishhack/libraft", "easyraft")

	lastError error
)

var holder = map[unsafe.Pointer]*raftServer{}
var counter = uint64(0)

func getSnapshot(r *raftServer) (ret []byte, err error) {
	var data unsafe.Pointer
	var size C.uint64_t
	res := C.GetSnapshotInternal(r.ctx, &data, &size)
	if res != 0 {
		err = fmt.Errorf("get snapshot failed[%d]", int(res))
	} else {
		ret = C.GoBytes(data, C.int(size))
		C.FreeSnapshotInternal(r.ctx, data)
	}
	return ret, err
}

func (r *raftServer) goAttach(f func()) {
	r.wgMu.RLock()
	defer r.wgMu.RUnlock()
	select {
	case <-r.stopc:
		plog.Warning("server has stopped (skipping goAttach)")
	default:
	}

	r.wg.Add(1)
	atomic.AddUint64(&r.attachCount, 1)
	go func() {
		defer r.wg.Done()
		f()
		atomic.AddUint64(&r.doneCount, 1)
	}()
}

func (r *raftServer) onStateReport() {
	defer plog.Notice("onStateReport exit")
	for {
		select {
		case <-r.stopc:
			return
		case s := <-r.inter.stateC:
			C.OnStateChangeInternal(r.ctx, C.int(s))
		}
	}
}

func (r *raftServer) readCommits() {
	for data := range r.inter.commitC {
		if data == nil {
			r.recoverFromSnapshot()
		} else {
			d := *data
			if len(d) > 0 {
				C.OnCommitInternal(r.ctx, unsafe.Pointer(&d[0]), C.uint64_t(len(d)))
			}
		}
	}
	if err, ok := <-r.inter.errorC; ok {
		plog.Fatal(err)
	}
	plog.Notice("readCommits exit")
}

func (r *raftServer) recoverFromSnapshot() {
	snapshot, err := r.snap.Load()
	if err == snap.ErrNoSnapshot {
		plog.Debugf("no snapshot")
		return
	}
	if err != nil && err != snap.ErrNoSnapshot {
		plog.Panic(err)
	}
	plog.Infof("loading snapshot at term %d and index %d", snapshot.Metadata.Term, snapshot.Metadata.Index)
	snapSize := len(snapshot.Data)
	var ret C.int
	if snapSize > 0 {
		ret = C.RecoverFromSnapshotInternal(r.ctx, unsafe.Pointer(&snapshot.Data[0]), C.uint64_t(snapSize))
	} else {
		ret = C.RecoverFromSnapshotInternal(r.ctx, null(), 0)
	}
	if ret != 0 {
		plog.Panic(fmt.Errorf("recover from snapshot failed[%d]", ret))
	}
}

const (
	purgeFileInterval = time.Duration(30) * time.Second
)

func (r *raftServer) purgeFile() {
	defer plog.Notice("purgeFile exit")
	var serrc, werrc <-chan error
	if r.cfg.MaxSnapFiles > 0 {
		plog.Infof("start purge snap file [maxFile: %d]\n", r.cfg.MaxSnapFiles)
		serrc = fileutil.PurgeFile(r.cfg.Snapdir, "snap", r.cfg.MaxSnapFiles, purgeFileInterval, r.stopc)
	}
	if r.cfg.MaxWALFiles > 0 {
		plog.Infof("start purge wal file [maxFile: %d]\n", r.cfg.MaxWALFiles)
		werrc = fileutil.PurgeFile(r.cfg.Waldir, "wal", r.cfg.MaxWALFiles, purgeFileInterval, r.stopc)
	}
	select {
	case e := <-serrc:
		plog.Fatalf("failed to purge snap file %v", e)
	case e := <-werrc:
		plog.Fatalf("failed to purge wal file %v", e)
	case <-r.stopc:
		return
	}
}

func (r *raftServer) onRaftStarted() {
	r.goAttach(r.purgeFile)
}

func null() unsafe.Pointer {
	return (unsafe.Pointer)(uintptr(0))
}

func ptr(i uint64) unsafe.Pointer {
	return (unsafe.Pointer)(uintptr(i))
}

//export GetVersion
func GetVersion(ver *C.char, n C.size_t) {
	version := C.CString("v1.0")
	defer C.free(unsafe.Pointer(version))
	C.strncpy(ver, version, n)
}

//export SetLogger
func SetLogger(logPath *C.char, debug C.int) C.int {
	name := C.GoString(logPath)
	if unsafe.Pointer(logPath) == null() {
		capnslog.SetFormatter(capnslog.NewNilFormatter())
		fmt.Println("change to nil formatter.")
	} else {
		var f *os.File
		var err error
		if name == "stdout" {
			f = os.Stdout
		} else if name == "stderr" {
			f = os.Stderr
		} else {
			f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				return 1
			}
		}
		capnslog.SetFormatter(capnslog.NewPrettyFormatter(f, debug != 0))
	}
	raft.SetLogger(plog)
	return 0
}

//export SetLogLevel
func SetLogLevel(logLevel C.int) C.int {
	plog.SetLevel(capnslog.LogLevel(logLevel))
	return 0
}

//export NewRaftServer
func NewRaftServer(ctx unsafe.Pointer, jsonConfig *C.char) unsafe.Pointer {
	svr := &raftServer{
		inter: &raftNodeInternal{
			proposeC:    make(chan []byte),
			snapshotC:   make(chan int),
			confChangeC: make(chan raftpb.ConfChange),
			stateC:      make(chan raft.StateType),
		},
		ctx:   ctx,
		stopc: make(chan struct{}),
	}
	svr.inter.ctx = svr
	svr.goAttach(svr.onStateReport)
	plog.Infof("NewRaftServer with config[%s]\n", C.GoString(jsonConfig))
	if err := json.Unmarshal([]byte(C.GoString(jsonConfig)), &svr.cfg); err != nil {
		lastError = err
		return null()
	}
	svr.node, svr.snap = newRaftNode(svr.cfg, svr.inter)
	svr.goAttach(svr.readCommits)
	cnt := ptr(atomic.AddUint64(&counter, 1))
	holder[cnt] = svr
	return cnt
}

//export DeleteRaftServer
func DeleteRaftServer(p unsafe.Pointer) {
	r := holder[p]
	if r != nil {
		plog.Debugf("before real stop, attach:%d done:%d", r.attachCount, r.doneCount)
		close(r.inter.proposeC)
		close(r.inter.confChangeC)
		r.node.stop()
		close(r.stopc)
		r.wg.Wait()
		delete(holder, p)
		runtime.GC()
	}
}

//export Propose
func Propose(p unsafe.Pointer, data unsafe.Pointer, size C.int) C.int {
	r := holder[p]
	if r != nil {
		r.inter.proposeC <- C.GoBytes(data, size)
		return 0
	}
	return 1
}

//export Snapshot
func Snapshot(p unsafe.Pointer) C.int {
	r := holder[p]
	if r != nil {
		r.inter.snapshotC <- 1
		return 0
	}
	return 1
}

func updateServer(updateType raftpb.ConfChangeType, p unsafe.Pointer, ID uint64, url *C.char) C.int {
	r := holder[p]
	if r != nil {
		r.inter.confChangeC <- raftpb.ConfChange{
			Type:    updateType,
			NodeID:  ID,
			Context: []byte(C.GoString(url)),
		}
		return 0
	}
	return 1
}

//export AddServer
func AddServer(p unsafe.Pointer, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeAddNode, p, ID, url)
}

//export DelServer
func DelServer(p unsafe.Pointer, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeRemoveNode, p, ID, url)
}

//export ChangeServer
func ChangeServer(p unsafe.Pointer, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeUpdateNode, p, ID, url)
}

func main() {}
