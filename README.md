# easyraft
RAFT C wrapper, origin go implement by coreos([Raft](https://github.com/coreos/etcd/tree/master/raft)) 

[![Coverity Scan Build Status](https://scan.coverity.com/projects/16077/badge.svg)](https://scan.coverity.com/projects/zxfishhack-easyraft)

## API
```c
// Set Global Raft Callback
DLL_EXPORTS void  RAFT_SetCallback(struct RAFT_Callback* ctx);
// Set Raft log
DLL_EXPORTS int   RAFT_SetLogger(const char* logPath, int debug);
// Set Raft log level
DLL_EXPORTS int   RAFT_SetLogLevel(int logLevel);
// Get Raft version
DLL_EXPORTS void  RAFT_GetVersion(char * v, size_t n);
// Create a raft node, using original wal/snap format
DLL_EXPORTS uint64_t RAFT_NewRaftServer(void* ctx, const char* jsonConfig);
// Delete a raft node
DLL_EXPORTS void  RAFT_DeleteRaftServer(uint64_t raft);
// Create a raft node, using wal/snap on rocksdb
DLL_EXPORTS uint64_t RAFT_NewRaftServerV2(void* ctx, const char* jsonConfig);
// Delete a raft node
DLL_EXPORTS void  RAFT_DeleteRaftServerV2(uint64_t raft, int leave);
// Propose a raft log
DLL_EXPORTS int   RAFT_Propose(uint64_t raft, void* data, int size, int timeoutms);
// Trigger raft node snapshot
DLL_EXPORTS int   RAFT_Snapshot(uint64_t raft);
// Add a raft node to cluster
DLL_EXPORTS int   RAFT_AddServer(uint64_t raft, uint64_t id, const char* url);
// Del a raft node from cluster
DLL_EXPORTS int   RAFT_DelServer(uint64_t raft, uint64_t id);
// Change a raft node url in cluster
DLL_EXPORTS int   RAFT_ChangeServer(uint64_t raft, uint64_t id, const char* url);
// Send a message to a raft node in cluster
DLL_EXPORTS int   RAFT_SendMessage(uint64_t raft, uint64_t id, const char* buf, size_t size, char* outbuf, size_t outsize);
```
## Usage

1. Setup callback with RAFT_SetCallback
1. Create Raft Server instance with RAFT_NewRaftServer(origin wal/snap format)/RAFT_NewRaftServerV2(wal/snap on rocksdb)
1. Using RAFT_Propose to do propose
1. Using RAFT_Snapshot force create a snapshot
1. Using RAFT_AddServer/RAFT_DelServer/RAFT_ChangeServer change cluster members
1. Explicit call RAFT_DeleteRaftServer/RAFT_DeleteRaftServerV2 before exit

## Callback
Callback|Usage|Return Value|Comment
--------|-----|------------|------
RAFT_Callback::getSnapshot|Raft State want a snapshot|0 meaning success|
RAFT_Callback::freeSnapshot|free the memory allocator by RAFT_Callback::getSnapshot||
RAFT_Callback::onStateChange|Raft State notify current state||
RAFT_Callback::recoverFromSnapshot|recovery server state with a snapshot|0 meaing success|WARNING: return non-zero will panic
RAFT_Callback::onCommit|notify a log is commited|0 meaning success|
RAFT_Callback::onMessage|notify when recveive a message||

## Config
```json
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
	"manual_snap": false,
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

manual_snap
>manual trigger snapshot if true

snapshot_entries
>keep how many log entries after snapshot

## Build & Test
>assume you have install go 1.9 & glide
### On Windows (not avaliable yet)
1. download tdm64-gcc-5.1.0-2.exe from http://tdm-gcc.tdragon.net/download
1. install & add tdm-gcc-64/bin to Windows PATH
1. run 'build.bat' build easyraft.dll
1. open tests\tests.sln with Visual Studio 2015
1. run bin\test.exe 1, bin\test.exe 2, bin\tests.exe 3
1. type help in console get command list

### On Linux
1. run 'make deps' get dependencies
1. run 'make makefile' build libeasyraft.so & tests
1. run ./bin/tests 1,./bin/tests 2, ./bin/tests 3
1. type help in console get command list
