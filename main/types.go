package main

import "C"
import "unsafe"
import "sync/atomic"
import "math/rand"

type onCommit func(ctx, ptr unsafe.Pointer, size C.size_t)
type onError func(ctx unsafe.Pointer, msg *C.char)
type persisterID uint64
type raftID uint64

func makeRaftID() *raftID {
	ret := new(raftID)
	*ret = raftID(rand.Uint64())
	return ret
}

func (id *raftID) get() raftID {
	return raftID(atomic.AddUint64((*uint64)(id), 1))
}

func makePersisterID() *persisterID {
	ret := new(persisterID)
	*ret = persisterID(rand.Uint64())
	return ret
}

func (id *persisterID) get() persisterID {
	return persisterID(atomic.AddUint64((*uint64)(id), 1))
}
