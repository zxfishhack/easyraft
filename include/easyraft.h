#ifndef _EASYRAFT_H_
#define _EASYRAFT_H_

#include <stdint.h>

struct Context {
    int (*getSnapshot)(void* ctx, void** data, uint64_t* size);
    void (*freeSnapshot)(void* ctx, void* data);
    void (*onStateChange)(void* ctx, int newState);
    int (*recoverFromSnapshot)(void* ctx, void* data, uint64_t size);
    int (*onCommit)(void* ctx, void* data, uint64_t size);
};

#ifdef _WIN32
#ifdef __GNUC__
#define DLL_EXPORTS 
#else
#define DLL_EXPORTS __declspec(dllimport)
#endif // __GNUC__ 
#else
#define DLL_EXPORTS
#endif // _WIN32

#ifdef __cplusplus
extern "C" {
#endif

#define STATE_FOLLOWER      0
#define STATE_CANDIDATE     1
#define STATE_LEADER        2
#define STATE_PRE_CANDIDATE 3

// CRITICAL is the lowest log level; only errors which will end the program will be propagated.
#define RAFT_LOG_CRITICAL -1
// ERROR is for errors that are not fatal but lead to troubling behavior.
#define RAFT_LOG_ERROR     0
// WARNING is for errors which are not fatal and not errors, but are unusual. Often sourced from misconfigurations.
#define RAFT_LOG_WARNING   1
// NOTICE is for normal but significant conditions.
#define RAFT_LOG_NOTICE    2
// INFO is a log level for common, everyday log updates.
#define RAFT_LOG_INFO      3
// DEBUG is the default hidden level for more verbose updates about internal processes.
#define RAFT_LOG_DEBUG     4
// TRACE is for (potentially) call by call tracing of programs.
#define RAFT_LOG_TRACE     5

DLL_EXPORTS void  RAFT_SetContext(struct Context* ctx);
DLL_EXPORTS int   RAFT_SetLogger(const char* logPath, int debug);
DLL_EXPORTS int   RAFT_SetLogLevel(int logLevel);
DLL_EXPORTS void  RAFT_GetVersion(char * v, size_t n);
DLL_EXPORTS void* RAFT_NewRaftServer(void* ctx, const char* jsonConfig);
DLL_EXPORTS void  RAFT_DeleteRaftServer(void* raft);
DLL_EXPORTS int   RAFT_Propose(void* raft, void* data, int size);
DLL_EXPORTS int   RAFT_Snapshot(void* raft);
DLL_EXPORTS int   RAFT_AddServer(void* raft, uint64_t id, const char* url);
DLL_EXPORTS int   RAFT_DelServer(void* raft, uint64_t id);
DLL_EXPORTS int   RAFT_ChangeServer(void* raft, uint64_t id, const char* url);

#ifdef __cplusplus
}
#endif

#endif
