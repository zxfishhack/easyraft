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

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/fileutil"

	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/rafthttp"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
)

type raftNodeInternal struct {
	proposeC    chan []byte
	snapshotC   chan int
	confChangeC chan raftpb.ConfChange
	commitC     chan *[]byte
	errorC      chan error
	stateC      chan raft.StateType
	ctx         *raftServer
}

type raftNode struct {
	inter *raftNodeInternal
	cfg   config

	lastIndex     uint64 // index of log at start
	confState     raftpb.ConfState
	snapshotIndex uint64
	appliedIndex  uint64

	node        raft.Node
	raftStorage *raft.MemoryStorage
	wal         *wal.WAL

	snapshotter      *snap.Snapshotter
	snapshotterReady chan *snap.Snapshotter

	snapCount uint64
	transport *rafthttp.Transport
	stopc     chan struct{}
	httpstopc chan struct{}
	httpdonec chan struct{}
}

func newRaftNode(cfg config, r *raftNodeInternal) (*raftNode, *snap.Snapshotter) {
	rc := &raftNode{
		inter:     r,
		cfg:       cfg,
		stopc:     make(chan struct{}),
		httpstopc: make(chan struct{}),
		httpdonec: make(chan struct{}),

		snapshotterReady: make(chan *snap.Snapshotter, 1),
	}
	rc.inter.commitC = make(chan *[]byte)
	rc.inter.errorC = make(chan error)

	rc.inter.ctx.goAttach(rc.startRaft)
	return rc, <-rc.snapshotterReady
}

func (rc *raftNode) saveSnap(snap raftpb.Snapshot) error {
	walSnap := walpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}
	if err := rc.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}
	if err := rc.snapshotter.SaveSnap(snap); err != nil {
		return err
	}
	return rc.wal.ReleaseLockTo(snap.Metadata.Index)
}

func (rc *raftNode) entriesToApply(ents []raftpb.Entry) (nents []raftpb.Entry) {
	if len(ents) == 0 {
		return
	}
	firstIdx := ents[0].Index
	if firstIdx > rc.appliedIndex+1 {
		plog.Fatalf("first index of commited entry[%d] should <= progress.appliedIndex[%d] + 1", firstIdx, rc.appliedIndex)
		return
	}
	if rc.appliedIndex-firstIdx+1 < uint64(len(ents)) {
		nents = ents[rc.appliedIndex-firstIdx+1:]
	}
	return
}

func (rc *raftNode) publishEntries(ents []raftpb.Entry) bool {
	if len(ents) > 0 {
		plog.Debugf("publishEntries [(%d,%d) -> (%d,%d)]",
			ents[0].Term, ents[0].Index, ents[len(ents)-1].Term, ents[len(ents)-1].Index)
	}
	for i := range ents {
		switch ents[i].Type {
		case raftpb.EntryNormal:
			if len(ents[i].Data) == 0 {
				// ignore empty messages
				break
			}
			select {
			case rc.inter.commitC <- &ents[i].Data:
			case <-rc.stopc:
				return false
			}
		case raftpb.EntryConfChange:
			var cc raftpb.ConfChange
			cc.Unmarshal(ents[i].Data)
			rc.confState = *rc.node.ApplyConfChange(cc)
			switch cc.Type {
			case raftpb.ConfChangeAddNode:
				if len(cc.Context) > 0 {
					rc.transport.AddPeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				}
			case raftpb.ConfChangeRemoveNode:
				if cc.NodeID == rc.cfg.ID {
					log.Println("I've been removed from the cluster! shutting down.")
					return false
				}
				rc.transport.RemovePeer(types.ID(cc.NodeID))
			case raftpb.ConfChangeUpdateNode:
				if len(cc.Context) > 0 {
					rc.transport.UpdatePeer(types.ID(cc.NodeID), []string{string(cc.Context)})
				}
			}
		}
		rc.appliedIndex = ents[i].Index
	}
	return true
}

func (rc *raftNode) loadSnapshot() *raftpb.Snapshot {
	snapshot, err := rc.snapshotter.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		plog.Fatalf("error loading snapshot (%v)", err)
		return nil
	}
	return snapshot
}

// openWAL returns a WAL ready for reading.
func (rc *raftNode) openWAL(snapshot *raftpb.Snapshot) *wal.WAL {
	if !wal.Exist(rc.cfg.Waldir) {
		if !fileutil.Exist(rc.cfg.Waldir) {
			if err := os.Mkdir(rc.cfg.Waldir, 0750); err != nil {
				plog.Fatalf("cannot create dir for wal (%v)", err)
			}
		}

		w, err := wal.Create(rc.cfg.Waldir, nil)
		if err != nil {
			plog.Fatalf("create wal error (%v)", err)
		}
		w.Close()
	}

	walsnap := walpb.Snapshot{}
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}
	plog.Infof("loading WAL at term %d and index %d", walsnap.Term, walsnap.Index)
	w, err := wal.Open(rc.cfg.Waldir, walsnap)
	if err != nil {
		plog.Fatalf("error loading wal (%v)", err)
	}

	return w
}

// replayWAL replays WAL entries into the raft instance.
func (rc *raftNode) replayWAL() *wal.WAL {
	plog.Printf("replaying WAL of member %d", rc.cfg.ID)
	snapshot := rc.loadSnapshot()
	w := rc.openWAL(snapshot)
	_, st, ents, err := w.ReadAll()
	if err != nil {
		plog.Fatalf("failed to read WAL (%v)", err)
	}
	rc.raftStorage = raft.NewMemoryStorage()
	if snapshot != nil {
		rc.raftStorage.ApplySnapshot(*snapshot)
		rc.inter.commitC <- nil
		plog.Debugf("sended snapshot(%d, %d)", snapshot.Metadata.Term, snapshot.Metadata.Index)
	}
	rc.raftStorage.SetHardState(st)

	// append to storage so raft starts at the right place in log
	rc.raftStorage.Append(ents)
	// send nil once lastIndex is published so client knows commit channel is current
	if len(ents) > 0 {
		rc.lastIndex = ents[len(ents)-1].Index
	}
	return w
}

func (rc *raftNode) writeError(err error) {
	rc.stopHTTP()
	close(rc.inter.commitC)
	rc.inter.errorC <- err
	close(rc.inter.errorC)
	rc.node.Stop()
}

func (rc *raftNode) startRaft() {
	if !fileutil.Exist(rc.cfg.Snapdir) {
		if err := os.Mkdir(rc.cfg.Snapdir, 0750); err != nil {
			plog.Fatalf("cannot create dir for snapshot (%v)", err)
		}
	}
	rc.snapshotter = snap.New(rc.cfg.Snapdir)
	rc.snapshotterReady <- rc.snapshotter

	haveWAL := wal.Exist(rc.cfg.Waldir)
	rc.wal = rc.replayWAL()

	rpeers := make([]raft.Peer, len(rc.cfg.Peers))
	for i := range rpeers {
		rpeers[i] = raft.Peer{
			ID: rc.cfg.Peers[i].ID,
		}
		plog.Debugf("cluster peer: %d", rpeers[i].ID)
	}
	c := &raft.Config{
		ID:              rc.cfg.ID,
		ElectionTick:    rc.cfg.ElectionTick,
		HeartbeatTick:   rc.cfg.HeartbeatTick,
		Storage:         rc.raftStorage,
		MaxSizePerMsg:   rc.cfg.MaxSizePerMsg,
		MaxInflightMsgs: rc.cfg.MaxInflightMsgs,
		Logger:          plog,
	}
	if haveWAL {
		rc.node = raft.RestartNode(c)
	} else {
		startPeers := rpeers
		if rc.cfg.JoinExist {
			startPeers = nil
		}
		plog.Debugf("start node peers:%v", startPeers)
		rc.node = raft.StartNode(c, startPeers)
	}
	rc.transport = &rafthttp.Transport{
		ID:          types.ID(rc.cfg.ID),
		ClusterID:   types.ID(rc.cfg.ClusterID),
		Raft:        rc,
		ServerStats: stats.NewServerStats("", ""),
		LeaderStats: stats.NewLeaderStats(strconv.Itoa(int(rc.cfg.ID))),
		ErrorC:      make(chan error),
	}
	rc.transport.Start()
	for _, peer := range rc.cfg.Peers {
		if peer.ID != rc.cfg.ID {
			rc.transport.AddPeer(types.ID(peer.ID), []string{peer.URL})
		}
	}

	rc.inter.ctx.goAttach(rc.serveRaft)
	rc.inter.ctx.goAttach(rc.serveChannels)
	plog.Notice("start raft done")
}

func (rc *raftNode) stop() {
	rc.stopHTTP()
	close(rc.inter.commitC)
	close(rc.inter.errorC)
	rc.node.Stop()
	plog.Notice("raft node stopped")
}

func (rc *raftNode) stopHTTP() {
	rc.transport.Stop()
	close(rc.httpstopc)
	<-rc.httpdonec
	plog.Notice("http server stopped")
}

func (rc *raftNode) publishSnapshot(snapshotToSave raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshotToSave) {
		return
	}

	plog.Infof("publishing snapshot at index %d", rc.snapshotIndex)
	defer plog.Infof("finished publishing snapshot at index %d", rc.snapshotIndex)

	if snapshotToSave.Metadata.Index <= rc.appliedIndex {
		plog.Fatalf("snapshot index [%d] should > progress.appliedIndex [%d] + 1", snapshotToSave.Metadata.Index, rc.appliedIndex)
	}
	rc.inter.commitC <- nil // trigger kvstore to load snapshot

	rc.confState = snapshotToSave.Metadata.ConfState
	rc.snapshotIndex = snapshotToSave.Metadata.Index
	rc.appliedIndex = snapshotToSave.Metadata.Index
}

func (rc *raftNode) maybeTriggerSnapshot(force bool) {
	if rc.appliedIndex-rc.snapshotIndex <= rc.cfg.SnapCount && !force {
		return
	}
	if rc.appliedIndex <= rc.snapshotIndex {
		return
	}
	plog.Infof("start snapshot [applied index: %d | last snapshot index: %d]", rc.appliedIndex, rc.snapshotIndex)
	data, err := getSnapshot(rc.inter.ctx)
	if err != nil {
		plog.Panic(err)
	}
	snap, err := rc.raftStorage.CreateSnapshot(rc.appliedIndex, &rc.confState, data)

	if err != nil {
		if err != raft.ErrSnapOutOfDate {
			return
		}
		panic(err)
	}
	if err := rc.saveSnap(snap); err != nil {
		panic(err)
	}
	rc.snapshotIndex = rc.appliedIndex
	compactIndex := uint64(1)
	if rc.appliedIndex > rc.cfg.SnapshotEntries {
		compactIndex = rc.appliedIndex - rc.cfg.SnapshotEntries
	}
	if err := rc.raftStorage.Compact(compactIndex); err != nil {
		if err == raft.ErrCompacted {
			return
		}
		panic(err)
	}

	plog.Infof("compacted log at index %d", compactIndex)

}

func (rc *raftNode) serveChannels() {
	defer plog.Notice("serveChannels exit")
	snap, err := rc.raftStorage.Snapshot()
	if err != nil {
		panic(err)
	}
	rc.confState = snap.Metadata.ConfState
	rc.snapshotIndex = snap.Metadata.Index
	rc.appliedIndex = snap.Metadata.Index

	defer rc.wal.Close()

	ticker := time.NewTicker(time.Duration(rc.cfg.TickMs) * time.Millisecond)
	defer ticker.Stop()

	// send proposals over raft
	rc.inter.ctx.goAttach(func() {
		var confChangeCount uint64
		defer plog.Notice("proposeC & confChangeC reader exit")

		for rc.inter.proposeC != nil && rc.inter.confChangeC != nil && rc.inter.snapshotC != nil {
			select {
			case prop, ok := <-rc.inter.proposeC:
				if !ok {
					rc.inter.proposeC = nil
				} else {
					// blocks until accepted by raft state machine
					rc.node.Propose(context.TODO(), []byte(prop))
				}

			case cc, ok := <-rc.inter.confChangeC:
				if !ok {
					rc.inter.confChangeC = nil
				} else {
					confChangeCount++
					cc.ID = confChangeCount
					rc.node.ProposeConfChange(context.TODO(), cc)
				}
			case _, ok := <-rc.inter.snapshotC:
				if !ok {
					rc.inter.snapshotC = nil
				} else {
					rc.maybeTriggerSnapshot(true)
				}
			}
		}
		// client closed channel; shutdown raft if not already

		close(rc.stopc)
	})

	// event loop on raft state machine updates
	for {
		select {
		case <-ticker.C:
			rc.node.Tick()

		// store raft entries to wal, then publish over commit channel
		case rd := <-rc.node.Ready():
			rc.wal.Save(rd.HardState, rd.Entries)
			if !raft.IsEmptySnap(rd.Snapshot) {
				rc.saveSnap(rd.Snapshot)
				rc.raftStorage.ApplySnapshot(rd.Snapshot)
				rc.publishSnapshot(rd.Snapshot)
			}
			rc.raftStorage.Append(rd.Entries)
			rc.transport.Send(rd.Messages)
			if ok := rc.publishEntries(rc.entriesToApply(rd.CommittedEntries)); !ok {
				rc.stop()
				return
			}
			rc.maybeTriggerSnapshot(false)
			rc.node.Advance()
			if rd.SoftState != nil {
				rc.inter.stateC <- rd.RaftState
			}
		case err := <-rc.transport.ErrorC:
			rc.writeError(err)
			return

		case <-rc.stopc:
			return
		}
	}
}

func (rc *raftNode) serveRaft() {
	peer := rc.cfg.getPeerByID(rc.cfg.ID)
	if peer == nil {
		plog.Fatal("cannot find self peer.")
	}
	url, err := url.Parse(peer.URL)
	if err != nil {
		plog.Fatalf("Failed parsing URL (%v)", err)
	}
	plog.Infof("start listen @ %v", url)
	ln, err := newStoppableListener(url.Host, rc.httpstopc)
	if err != nil {
		plog.Fatalf("Failed to listen rafthttp (%v)", err)
	}

	err = (&http.Server{Handler: rc.transport.Handler()}).Serve(ln)
	select {
	case <-rc.httpstopc:
	default:
		plog.Fatalf("Failed to serve rafthttp (%v)", err)
	}
	close(rc.httpdonec)
	plog.Notice("serveRaft exit")
}

func (rc *raftNode) Process(ctx context.Context, m raftpb.Message) error {
	return rc.node.Step(ctx, m)
}
func (rc *raftNode) IsIDRemoved(id uint64) bool                           { return false }
func (rc *raftNode) ReportUnreachable(id uint64)                          {}
func (rc *raftNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}
