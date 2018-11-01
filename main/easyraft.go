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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/coreos/etcd/pkg/fileutil"

	"github.com/coreos/etcd/raft"
	snap "github.com/coreos/etcd/raftsnap"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/pkg/capnslog"

	"../stablestorage"
)

type PeerStatus struct {
	ID         uint64 `json:"id"`
	Host       string `json:"host"`
	ActiveTime int    `json:"active"`
}

type raftServer struct {
	inter *raftNodeInternal
	snap  stablestorage.Snapshotter
	ctx   unsafe.Pointer
	node  *raftNode
	wgMu  sync.RWMutex
	wg    sync.WaitGroup
	cfg   config
	stopc chan struct{}
	stor  stablestorage.StableStorage

	// for debug
	attachCount uint64
	doneCount   uint64
}

var (
	plog = capnslog.NewPackageLogger("github.com/zxfishhack/libraft", "easyraft")

	lastError error
)

var holder = map[C.uint64_t]*raftServer{}
var counter = uint64(1)

func null() unsafe.Pointer {
	return (unsafe.Pointer)(uintptr(0))
}

func getSnapshot(r *raftServer) (ret []byte, err error) {
	var data unsafe.Pointer
	var size C.uint64_t
	res := C.GetSnapshotInternal(r.ctx, &data, &size)
	if res != 0 {
		err = fmt.Errorf("get snapshot failed[%d]", int(res))
	} else {
		ret = C.GoBytes(data, C.int(size))
		C.FreeInternal(r.ctx, data)
	}
	return ret, err
}

func processMessage(r *raftServer, w http.ResponseWriter, req *http.Request) {
	var data unsafe.Pointer
	var size C.uint64_t
	d, _ := ioutil.ReadAll(req.Body)
	res := C.OnMessageInternal(r.ctx, unsafe.Pointer(&d[0]), C.uint64_t(len(d)), &data, &size)
	if res != 0 {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(C.GoBytes(data, C.int(size)))
		C.FreeInternal(r.ctx, data)
	}
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
	defer plog.Notice("readCommits exit")
	for ent := range r.inter.commitC {
		if ent == nil {
			r.recoverFromSnapshot()
		} else {
			d := ent.Data
			if len(d) > 0 {
				C.OnCommitInternal(r.ctx, unsafe.Pointer(&d[0]), C.uint64_t(len(d)), C.uint64_t(ent.Term), C.uint64_t(ent.Index))
			}
		}
	}
	if err, ok := <-r.inter.errorC; ok {
		plog.Fatal(err)
	}
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
		ret = C.RecoverFromSnapshotInternal(r.ctx, unsafe.Pointer(&snapshot.Data[0]), C.uint64_t(snapSize), C.uint64_t(snapshot.Metadata.Term), C.uint64_t(snapshot.Metadata.Index))
	} else {
		ret = C.RecoverFromSnapshotInternal(r.ctx, null(), 0, C.uint64_t(snapshot.Metadata.Term), C.uint64_t(snapshot.Metadata.Index))
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

func (r *raftServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	processMessage(r, w, req)
}

func (r *raftServer) onRaftStarted() {
	if r.stor == nil {
		r.goAttach(r.purgeFile)
	}
}

//export GetVersion
func GetVersion(ver *C.char, n C.size_t) {
	version := C.CString("v3.0")
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
func NewRaftServer(ctx unsafe.Pointer, jsonConfig *C.char) C.uint64_t {
	svr := &raftServer{
		inter: &raftNodeInternal{
			proposeC:    make(chan []byte),
			snapshotC:   make(chan context.Context),
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
		return C.uint64_t(0)
	}
	svr.node, svr.snap = newRaftNode(svr.cfg, svr.inter)
	svr.goAttach(svr.readCommits)
	cnt := C.uint64_t(atomic.AddUint64(&counter, 1))
	holder[cnt] = svr
	svr.inter.initDone.Wait()
	return cnt
}

//export DeleteRaftServer
func DeleteRaftServer(p C.uint64_t) {
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

//export NewRaftServerV2
func NewRaftServerV2(ctx unsafe.Pointer, jsonConfig *C.char) C.uint64_t {
	svr := &raftServer{
		inter: &raftNodeInternal{
			proposeC:    make(chan []byte),
			snapshotC:   make(chan context.Context),
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
		return C.uint64_t(0)
	}
	var err error
	svr.stor, err = stablestorage.NewRocksdbStorage(svr.cfg.StoragePath)
	if svr.stor == nil {
		lastError = err
		return C.uint64_t(0)
	}

	svr.node, svr.snap = newRaftNodeV2(svr.cfg, svr.inter, svr.stor)
	svr.goAttach(svr.readCommits)
	cnt := C.uint64_t(atomic.AddUint64(&counter, 1))
	holder[cnt] = svr
	svr.inter.initDone.Wait()
	return cnt
}

//export DeleteRaftServerV2
func DeleteRaftServerV2(p C.uint64_t, purge C.int) {
	r := holder[p]
	if r != nil {
		plog.Debugf("before real stop, attach:%d done:%d", r.attachCount, r.doneCount)
		close(r.inter.proposeC)
		close(r.inter.confChangeC)
		r.node.stop()
		close(r.stopc)
		r.wg.Wait()
		delete(holder, p)
		if purge == 1 && r.wal != nil {
			r.wal.Purge()
		}
		runtime.GC()
	}
}

//export Propose
func Propose(p C.uint64_t, data unsafe.Pointer, size C.int, timeoutms C.int) C.int {
	r := holder[p]
	if r != nil {
		select {
		case r.inter.proposeC <- C.GoBytes(data, size):
		case <-time.After(time.Duration(int(timeoutms)) * time.Millisecond):
			//TODO:exit?
			plog.Errorf("Propose timeout")
			return 2
		}
		return 0

	}
	plog.Error("Propose to unknown RAFT state.")
	return 1
}

//export Snapshot
func Snapshot(p C.uint64_t) C.int {
	r := holder[p]
	done := make(chan error)
	ctx := context.WithValue(context.TODO(), "done", done)
	if r != nil {
		r.inter.snapshotC <- ctx
	}
	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			plog.Error("snapshot error", ctx.Err())
			return 1
		}
	case err := <-done:
		if err != nil {
			plog.Error("snapshot error", err)
			return 1
		}
	}
	return 0
}

func updateServer(updateType raftpb.ConfChangeType, p C.uint64_t, ID uint64, url *C.char) C.int {
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
func AddServer(p C.uint64_t, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeAddNode, p, ID, url)
}

//export DelServer
func DelServer(p C.uint64_t, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeRemoveNode, p, ID, url)
}

//export ChangeServer
func ChangeServer(p C.uint64_t, ID uint64, url *C.char) C.int {
	return updateServer(raftpb.ConfChangeUpdateNode, p, ID, url)
}

//export GetPeersStatus
func GetPeersStatus(p C.uint64_t, buf *C.char, size C.size_t) C.int {
	r := holder[p]
	if r != nil {
		status := r.node.getPeersStatus()
		js, err := json.Marshal(status)
		if err != nil {
			return 2
		}
		jsb := C.CString(string(js))
		defer C.free(unsafe.Pointer(jsb))
		if int(size) <= len(js) {
			return 3
		}
		C.strncpy(buf, jsb, size)
		return 0
	}
	return 1
}

//export GetStatus
func GetStatus(p C.uint64_t, buf *C.char, size C.size_t) C.int {
	r := holder[p]
	if r != nil {
		status := r.node.node.Status().String()
		str := C.CString(status)
		defer C.free(unsafe.Pointer(str))
		if int(size) < len(status) {
			return 3
		}
		C.strncpy(buf, str, size)
		return 0
	}
	return 1
}

//export SendMessage
func SendMessage(p C.uint64_t, ID uint64, buf *C.char, size C.size_t, outbuf *C.char, outsize C.size_t) int {
	r := holder[p]
	if r != nil {
		if ID == r.cfg.ID {
			return -2
		}
		var url *string
		for _, p := range r.cfg.Peers {
			if p.ID == ID {
				url = &p.URL
				break
			}
		}
		if url == nil {
			return -3
		}
		body := bytes.NewBuffer(C.GoBytes(unsafe.Pointer(buf), C.int(size)))
		req, err := http.NewRequest("POST", (*url)+"/message", body)
		if err != nil {
			return -4
		}
		req.Header.Set("X-Etcd-Cluster-ID", strconv.FormatUint(r.cfg.ClusterID, 10))
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return -5
		}
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return -6
		}
		if int(outsize) < len(respBody) {
			return -1
		}
		if len(respBody) > 0 {
			C.memcpy(unsafe.Pointer(outbuf), unsafe.Pointer(&respBody[0]), C.size_t(len(respBody)))
		}
		return len(respBody)
	}
	return -2
}

func main() {}
