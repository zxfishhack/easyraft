package main

import (
	/*
		#include <stdlib.h>
		#include <string.h>
	*/
	"C"
	"fmt"
	"unsafe"
)

var persisterIds *persisterID
var persisters map[persisterID]*persister
var raftIds *raftID
var rafts map[raftID]*raftNode

//export RAFTVersion
func RAFTVersion(ver *C.char, n C.size_t) {
	version := C.CString("v1.0")
	defer C.free(unsafe.Pointer(version))
	fmt.Println(ver, n)
	C.strncpy(ver, version, n)
}

//export RAFTInit
func RAFTInit() bool {
	persisters = make(map[persisterID]*persister)
	rafts = make(map[raftID]*raftNode)
	persisterIds = makePersisterID()
	raftIds = makeRaftID()
	return true
}

//export RAFTUinit
func RAFTUinit() {
	persisters = nil
	rafts = nil
	raftIds = nil
	persisterIds = nil
}

//export PersisterNew
func PersisterNew() persisterID {
	return persisterIds.get()
}

//export PersisterDelete
func PersisterDelete(id persisterID) {
	delete(persisters, id)
}

//export RAFTNew
func RAFTNew() raftID {
	return raftIds.get()
}

//export RAFTDelete
func RAFTDelete(id raftID) {
	delete(rafts, id)
}

//export RAFTPropose
func RAFTPropose(id raftID, ptr unsafe.Pointer, size C.size_t) bool {
	return false
}

//export RAFTLogSize
func RAFTLogSize(id raftID) uint64 {
	return 0
}

func main() {}
