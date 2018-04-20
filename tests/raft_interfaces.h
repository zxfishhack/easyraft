#pragma once
#include <cstdint>
#include <vector>
#include <string>
#include <mutex>
#include <algorithm>
#include <condition_variable>

struct propose {
	uint64_t id;
	uint64_t cid;
	uint64_t seq;
	char cmd[1];
	constexpr static size_t header_length() {
		return sizeof(propose) - sizeof(cmd);
	}
};

class proposeWaiter {
public:
	proposeWaiter() : seq_(0) {}
	void wait(uint64_t seq) {
		std::unique_lock<std::mutex> lck(mu_);
		cv_.wait(lck, [&]()->bool { return seq_ >= seq; });
	}
	void signal(uint64_t seq) {
		std::unique_lock<std::mutex> lck(mu_);
		seq_ = std::max(static_cast<uint64_t>(seq_), seq);
		cv_.notify_all();
	}
	uint64_t getSeq() {
		std::unique_lock<std::mutex> lck(mu_);
		return ++seq_;
	}
private:
	std::mutex mu_;
	std::condition_variable cv_;
	volatile uint64_t seq_;
};

void splitCmd(const std::string& line, std::string& cmd, uint64_t& cid, std::vector<std::string>& arg);
int getSnapshot(void* ctx, void** data, uint64_t* size);
void freeSnapshot(void* ctx, void* data);
void onStateChange(void* ctx, int newState);
int recoverFromSnapshot(void* ctx, void* data, uint64_t size, uint64_t term, uint64_t index);
int onCommit(void* ctx, void* data, uint64_t size, uint64_t term, uint64_t index);
int onMessage(void* ctx, void* data, uint64_t size, void**outdata, uint64_t* outsize);
