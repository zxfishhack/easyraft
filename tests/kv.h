#pragma once
#include <string>
#include <sstream>
#include <mutex>
#include <map>
#include "endian_rw.h"

class kv {
public:
	bool get(const std::string& k, std::string& v) {
		std::unique_lock<std::mutex> lck(mu_);
		auto it = kv_.find(k);
		if (it == kv_.end()) {
			return false;
		}
		v = it->second;
		return true;
	}
	void put(const std::string& k, const std::string& v) {
		std::unique_lock<std::mutex> lck(mu_);
		kv_[k] = v;
	}
	void app(const std::string& k, const std::string& v) {
		std::unique_lock<std::mutex> lck(mu_);
		kv_[k] += v;
	}
	void app(const std::string& k, const std::string& v, std::string& nv) {
		std::unique_lock<std::mutex> lck(mu_);
		kv_[k] += v;
		nv = kv_[k];
	}
	void del(const std::string& k) {
		std::unique_lock<std::mutex> lck(mu_);
		kv_.erase(k);
	}
	void serialize(std::string& s) {
		std::unique_lock<std::mutex> lck(mu_);
		std::ostringstream os;
		os < kv_.size();
		for (auto& kv : kv_) {
			os < kv.first.length();
			os < kv.first;
			os < kv.second.length();
			os < kv.second;
		}
		s = os.str();
	}
	void deserialize(std::string& s) {
		std::unique_lock<std::mutex> lck(mu_);
		kv_.clear();
		std::istringstream is(s);
		size_t cnt;
		try {
			is < cnt;
			std::string k, v;
			size_t c;
			for(size_t i=0; i<cnt; i++) {
				is < c;
				isread(is, k, c);
				is < c;
				isread(is, v, c);
				kv_[k] = v;
			}
		} catch(eof) {
			std::cout << "deserialize maybe failed, eof before read all data." << std::endl;
		}
	}
private:
	std::mutex mu_;
	std::map<std::string, std::string> kv_;
};
