#include "raft_interfaces.h"
#include "kv.h"

#include <iostream>
#include <vector>
#include <atomic>
#include <cstring>
#include "chan.h"

extern kv g_kv;
extern proposeWaiter g_pw;
extern uint64_t self;
extern std::atomic<uint64_t> g_done;
extern chan<int> g_joinC;

bool foo() {
	return true;
}

bool test = foo();

const char* raftState[] = {
	"StateFollower",
	"StateCandidate",
	"StateLeader",
	"StatePreCandidate"
};

int getSnapshot(void* ctx, void** data, uint64_t* size) {
	std::string s;
	g_kv.serialize(s);
	*size = s.length();
	*data = malloc(s.length());
	memcpy(*data, s.data(), *size);
	return 0;
}

void freeSnapshot(void* ctx, void* data) {
	if (data) {
		free(data);
	}
}


void onStateChange(void* ctx, int newState) {
	if (newState < 0 || newState >= sizeof(raftState)) {
		std::cout << "got wrong state " << newState << std::endl;
	}
	else {
		std::cout << "current state " << raftState[newState] << std::endl;
	}
}

int recoverFromSnapshot(void* ctx, void* data, uint64_t size) {
	std::cout << "recoverFromSnapshot\n";
	std::string str((char*)data, size);
	g_kv.deserialize(str);
	int t;
	g_joinC.recv(t);
	return 0;
}

int onCommit(void* ctx, void* data, uint64_t size) {
	auto pr = static_cast<propose*>(data);
	std::string line;
	line.assign(pr->cmd, size - propose::header_length());
	if (pr->id == self) {
		++g_done;
	}
	if (strncmp(line.c_str(), "bench", 5) == 0) {
		return 0;
	}
	std::string cmd;
	std::vector<std::string> arg;
	splitCmd(line, cmd, arg);
	if (cmd == "set" && arg.size() >= 2u) {
		g_kv.put(arg[0], arg[1]);
	}
	else if (cmd == "app" && arg.size() >= 2u) {
		g_kv.app(arg[0], arg[1]);
	}
	else if (cmd == "del" && arg.size() > 1u) {
		g_kv.del(arg[0]);
	}
	if (pr->id == self) {
		g_pw.signal(pr->seq);
	}
	return 0;
}
