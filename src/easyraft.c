#include <libeasyraft.h>

void RAFT_SetContext(struct Context* ctx) {
    context = *ctx;
}

int RAFT_SetLogger(const char* logPath, int debug) {
    return SetLogger((char*)logPath, debug);
}

int RAFT_SetLogLevel(int logLevel) {
    return SetLogLevel(logLevel);
}

void RAFT_GetVersion(char * v, size_t n) {
    GetVersion(v, n);
}

void* RAFT_NewRaftServer(void* ctx, const char* jsonConfig) {
    return NewRaftServer(ctx, (char*)jsonConfig);
}

void RAFT_DeleteRaftServer(void* raft) {
    return DeleteRaftServer(raft);
}

int RAFT_Propose(void* raft, void* data, int size) {
    return Propose(raft, data, size);
}

int RAFT_Snapshot(void* raft) {
    return Snapshot(raft);
}

int RAFT_AddServer(void* raft, uint64_t id, const char* url) {
    return AddServer(raft, id, (char*)url);
}

int RAFT_DelServer(void* raft, uint64_t id) {
    return DelServer(raft, id, (char*)"");
}

int RAFT_ChangeServer(void* raft, uint64_t id, const char* url) {
    return ChangeServer(raft, id, (char*)url);
}
