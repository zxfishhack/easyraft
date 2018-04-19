package main

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
)

type raftHttpHandler struct {
	svr      *http.Server
	mux      *http.ServeMux
	mu       sync.RWMutex
	handlers map[uint64]http.Handler
	ln       *stoppableListener
	stopc    chan struct{}
	donec    chan struct{}
}

func (rh *raftHttpHandler) AddHandler(cid uint64, h http.Handler) error {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	if _, ok := rh.handlers[cid]; ok {
		return os.ErrExist
	}
	rh.handlers[cid] = h
	return nil
}

func (rh *raftHttpHandler) RemoveHandler(cid uint64) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	delete(rh.handlers, cid)
}

func (rh *raftHttpHandler) Destory() error {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	for _, _ = range rh.handlers {
		return os.ErrPermission
	}
	close(rh.stopc)
	select {
	case <-rh.donec:
	default:
	}
	return nil
}

func (rh *raftHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gcid, err := strconv.ParseUint(r.Header.Get("X-Etcd-Cluster-ID"), 10, 64)
	var h http.Handler
	rh.mu.RLock()
	if err != nil {
		plog.Debug(r.URL)
		for _, v := range rh.handlers {
			h = v
			break
		}
	} else if v, ok := rh.handlers[gcid]; ok {
		h = v
	}
	rh.mu.RUnlock()
	if h != nil {
		h.ServeHTTP(w, r)
	}
}

func newRaftHttpHandler(url *url.URL) (rh *raftHttpHandler, err error) {
	plog.Infof("start listen @ %v", url)

	rh = &raftHttpHandler{
		handlers: make(map[uint64]http.Handler),
		stopc:    make(chan struct{}, 1),
		donec:    make(chan struct{}, 1),
	}
	rh.ln, err = newStoppableListener(url.Host, rh.stopc)
	if err != nil {
		return nil, err
	}
	rh.mux = http.NewServeMux()
	rh.mux.Handle("/raft", rh)
	rh.mux.Handle("/snapshots", http.StripPrefix("/snapshots", http.FileServer(http.Dir("./snapshots"))))
	rh.svr = &http.Server{
		Handler: rh.mux,
	}
	
	go func() {
		rh.svr.Serve(rh.ln)
		select {
		case <-rh.stopc:
		default:
			plog.Fatalf("Failed to serve rafthttp (%v)", err)
		}
		close(rh.donec)
	}()
	return rh, err
}

type objHolder struct {
	v  interface{}
	c  int64
	cf func(v interface{})
}

var objs map[string]*objHolder
var objMtx sync.RWMutex
var objOnce sync.Once

func lookup(k string) interface{} {
	objMtx.RLock()
	defer objMtx.RUnlock()
	if obj, ok := objs[k]; ok {
		obj.c = obj.c + 1
		return obj.v
	}
	return nil
}

func add(k string, v interface{}, cf func(v interface{})) interface{} {
	tmp := &objHolder{
		v:  v,
		c:  0,
		cf: cf,
	}
	objMtx.Lock()
	defer objMtx.Unlock()
	objs[k] = tmp
	objs[k].c = 1
	return tmp.v
}

func release(k string) {
	objMtx.Lock()
	defer objMtx.Unlock()
	if obj, ok := objs[k]; ok {
		obj.c = obj.c - 1
		if obj.c == 0 {
			delete(objs, k)
			plog.Infof("release %v", k)
		}
	}
}

func GetRaftHttpHandler(url *url.URL) (rh *raftHttpHandler, err error) {
	objOnce.Do(func() {
		objs = make(map[string]*objHolder)
	})
	if r := lookup(url.Host); r != nil {
		return r.(*raftHttpHandler), nil
	}
	rh, err = newRaftHttpHandler(url)
	if err != nil {
		return nil, err
	}
	return add(url.Host, rh, func(v interface{}) {
		v.(*raftHttpHandler).Destory()
	}).(*raftHttpHandler), nil
}

func ReleaseRaftHttpHandler(url *url.URL) {
	release(url.Host)
}
