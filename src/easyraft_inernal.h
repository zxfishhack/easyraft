#ifndef _EASYRAFT_INTERNAL_H_
#define _EASYRAFT_INTERNAL_H_

#include "easyraft.h"

extern struct RAFT_Callback cb;

#ifdef _CGO_BUILD_
static int GetSnapshotInternal(void* ctx, void** data, uint64_t* size) {
    return cb.getSnapshot(ctx, data, size);
}
static void FreeSnapshotInternal(void* ctx, void* data) {
    return cb.freeSnapshot(ctx, data);
}
static void OnStateChangeInternal(void* ctx, int newState) {
    cb.onStateChange(ctx, newState);
}

static int RecoverFromSnapshotInternal(void* ctx, void* data, uint64_t size) {
    return cb.recoverFromSnapshot(ctx, data, size);
}

static int OnCommitInternal(void* ctx, void* data, uint64_t size) {
    return cb.onCommit(ctx, data, size);
}

struct RAFT_Callback cb;
#endif

#endif