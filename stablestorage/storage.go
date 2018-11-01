package stablestorage

import (
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal/walpb"
)

type WAL interface {
	ReadAll() (metadata []byte, state raftpb.HardState, ents []raftpb.Entry, err error)
	ReleaseLockTo(index uint64) error
	Close() error
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	SaveSnapshot(e walpb.Snapshot) error
}

type Snapshotter interface {
	SaveSnap(snapshot raftpb.Snapshot) error
	Load() (*raftpb.Snapshot, error)
}

type StableStorage interface {
	GetWAL(clusterId, nodeId uint64) (WAL, error)
	ExistWAL(clusterId, nodeId uint64) bool
	GetSnapshooter(clusterId, nodeId uint64) Snapshotter
}