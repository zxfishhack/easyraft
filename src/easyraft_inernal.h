#ifndef _EASYRAFT_INTERNAL_H_
#define _EASYRAFT_INTERNAL_H_

#include "easyraft.h"

extern struct RAFT_Callback cb;

#ifdef _CGO_BUILD_
static int GetSnapshotInternal(void* ctx, void** data, uint64_t* size) {
    return cb.getSnapshot(ctx, data, size);
}
static void FreeInternal(void* ctx, void* data) {
    return cb.free(ctx, data);
}
static void OnStateChangeInternal(void* ctx, int newState) {
    cb.onStateChange(ctx, newState);
}

static int RecoverFromSnapshotInternal(void* ctx, void* data, uint64_t size, uint64_t term, uint64_t index) {
    return cb.recoverFromSnapshot(ctx, data, size, term, index);
}

static int OnCommitInternal(void* ctx, void* data, uint64_t size, uint64_t term, uint64_t index) {
    return cb.onCommit(ctx, data, size, term, index);
}

static int OnMessageInternal(void* ctx, void* data, uint64_t size, void**outdata, uint64_t* outsize) {
    return cb.onMessage(ctx, data, size, outdata, outsize);
}

struct RAFT_Callback cb;
#endif

#endif