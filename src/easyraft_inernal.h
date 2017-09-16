#ifndef _EASYRAFT_INTERNAL_H_
#define _EASYRAFT_INTERNAL_H_

#include "easyraft.h"

extern struct Context context;

#ifdef _CGO_BUILD_
static int GetSnapshotInternal(void* ctx, void** data, uint64_t* size) {
    return context.getSnapshot(ctx, data, size);
}
static void FreeSnapshotInternal(void* ctx, void* data) {
    return context.freeSnapshot(ctx, data);
}
static void OnStateChangeInternal(void* ctx, int newState) {
    context.onStateChange(ctx, newState);
}

static int RecoverFromSnapshotInternal(void* ctx, void* data, uint64_t size) {
    return context.recoverFromSnapshot(ctx, data, size);
}

static int OnCommitInternal(void* ctx, void* data, uint64_t size) {
    return context.onCommit(ctx, data, size);
}

struct Context context;
#endif

#endif