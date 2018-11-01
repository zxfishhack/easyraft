#include <libeasyraft.h>

void RAFT_SetCallback(struct RAFT_Callback* ctx) {
    cb = *ctx;
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

uint64_t RAFT_NewRaftServer(void* ctx, const char* jsonConfig) {
    return NewRaftServer(ctx, (char*)jsonConfig);
}

void RAFT_DeleteRaftServer(uint64_t raft) {
    return DeleteRaftServer(raft);
}

uint64_t RAFT_NewRaftServerV2(void* ctx, const char* jsonConfig) {
    return NewRaftServerV2(ctx, (char*)jsonConfig);
}

void RAFT_DeleteRaftServerV2(uint64_t raft, int purge) {
    return DeleteRaftServerV2(raft, purge);
}

int RAFT_Propose(uint64_t raft, void* data, int size, int timeoutms) {
    return Propose(raft, data, size, timeoutms);
}

int RAFT_Snapshot(uint64_t raft) {
    return Snapshot(raft);
}

int RAFT_AddServer(uint64_t raft, uint64_t id, const char* url) {
    return AddServer(raft, id, (char*)url);
}

int RAFT_DelServer(uint64_t raft, uint64_t id) {
    return DelServer(raft, id, (char*)"");
}

int RAFT_ChangeServer(uint64_t raft, uint64_t id, const char* url) {
    return ChangeServer(raft, id, (char*)url);
}

int RAFT_GetPeersStatus(uint64_t raft, char* buf, size_t size) {
    return GetPeersStatus(raft, buf, size);
}

int RAFT_GetStatus(uint64_t raft, char* buf, size_t size) {
    return GetStatus(raft, buf, size);
}

int RAFT_SendMessage(uint64_t raft, uint64_t id, const char* buf, size_t size, char* outbuf, size_t outsize) {
    return SendMessage(raft, id, (char*)buf, size, outbuf, outsize);
}
