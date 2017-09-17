# easyraft
RAFT C wrapper implement from coreos/etcd 

## API
```
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
```
## Usage

1. Setup callback context with RAFT_SetContext
1. Create Raft Server instance with RAFT_NewRaftServer
1. Using RAFT_Propose to do propose
1. Using RAFT_Snapshot force create a snapshot
1. Using RAFT_AddServer/RAFT_DelServer/RAFT_ChangeServer change cluster members
1. Explicit call RAFT_DeleteRaftServer before exit

## Callback
Callback|Usage|Return Value|Comment
--------|-----|------------|------
Context::getSnapshot|Raft State want a snapshot|0 meaning success|
Context::freeSnapshot|free the memory allocator by Context::getSnapshot||
Context::onStateChange|Raft State notify current state||
Context::recoverFromSnapshot|recovery server state with a snapshot|0 meaing success|WARNING: return non-zero will panic
Context::onCommit|notify a log is commited|0 meaning success|

## Config
```
{
	"id" : 1,
	"cluster_id" : 1,
	"snap_count" : 10000,
	"waldir" : "wal1",
	"snapdir" : "snap1",
	"tickms" : 100,
	"election_tick" : 10,
	"heartbeat_tick" : 1,
	"boostrap_timeout" : 1,
	"peers" : [{
			"id" : 1,
			"url" : "http://127.0.0.1:9001"
		}, {
			"id" : 2,
			"url" : "http://127.0.0.1:9002"
		}, {
			"id" : 3,
			"url" : "http://127.0.0.1:9003"
		}
	],
	"join" : false,
	"max_size_per_msg" : 1048576,
	"max_inflight_msgs" : 256,
	"snapshot_entries" : 1000
}
```
id
>ID of the node. An ID represents a unique node in a cluster for all time. A given ID MUST be used only once even if the old node has been removed. This means that for example IP addresses make poor node IDs since they may be reused. Node IDs must be non-zero.

cluster_id
>cluster identify

snap_count
>how many log count will auto trigger snapshot

tickms
>Raft state machine tick interval(in ms) 

election_tick
>election timeout tick

heartbeat_tick
>tick count between heartbeat

boostrap_timeout
>boostrap timeout(in second)

peers
>all nodes for cluster setup

join
>a new node join a cluster

max_size_per_msg
>the max size of each append message, recommand 

max_inflight_msgs
>the max number of in-flight append messages during optimistic replication phase

snapshot_entries
>keep how many log entries after snapshot
