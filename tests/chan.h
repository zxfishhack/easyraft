#pragma once
#include <mutex>
#include <condition_variable>
#include <deque>

template<typename T>
class chan {
public:
	chan() : m_closed(false) {}
	~chan() {
		close();
	}
	bool hasData()
	{
		return m_queue.empty();
	}
	bool isClosed()
	{
		return m_closed;
	}
	bool recv(T& v) {
		std::unique_lock<std::mutex> lck(m_mutex);
		while (m_queue.empty() && !m_closed) {
			m_cv.wait(lck);
		}
		if (m_closed) {
			return false;
		}
		v = m_queue.front();
		m_queue.pop_front();
		return true;
	}
	bool send(const T& v) {
		if (m_closed) {
			return false;
		}
		std::unique_lock<std::mutex> lck(m_mutex);
		auto needSignal = m_queue.empty();
		m_queue.push_back(v);
		if (needSignal) {
			m_cv.notify_one();
		}
		return true;
	}
	void close() {
		std::unique_lock<std::mutex> lck(m_mutex);
		m_closed = true;
		m_cv.notify_all();
	}
private:
	volatile bool m_closed;
	std::deque<T> m_queue;
	std::mutex m_mutex;
	std::condition_variable m_cv;
};