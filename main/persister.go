package main

import (

	/*
		#include <string.h>
		typedef void(*getSnapshot)(void*, void**, size_t*);
		typedef void(*releaseSnapshot)(void*, void*, size_t);
		void getssAdapter(getSnapshot func, void* ctx, void** ptr, size_t *size) {
			func(ctx, ptr, size);
		}
		void delssAdapter(releaseSnapshot func, void* ctx, void* ptr, size_t size) {
			func(ctx, ptr, size);
		}
	*/
	"C"
	"unsafe"
)

type persister struct {
	ctx   unsafe.Pointer
	getss C.getSnapshot
	delss C.releaseSnapshot
}

func (p *persister) getSnapshot() ([]byte, error) {
	ptr := new(unsafe.Pointer)
	size := new(C.size_t)

	C.getssAdapter(p.getss, p.ctx, ptr, size)
	ret := make([]byte, *size)
	if *size > 0 {
		C.memcpy(unsafe.Pointer(&ret[0]), *ptr, *size)
		C.delssAdapter(p.delss, p.ctx, *ptr, *size)
	}
	return ret, nil
}
