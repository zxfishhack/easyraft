package main

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
)

type raftHttpHandler struct {
	svr *http.Server
	mu sync.RWMutex
	handlers map[uint64]http.Handler
	ln *stoppableListener
	stopc chan struct{}
	donec chan struct{}
}

func (rh *raftHttpHandler)AddHandler(cid uint64, h http.Handler) error {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	if _, ok := rh.handlers[cid]; ok {
		return os.ErrExist
	}
	rh.handlers[cid] = h
}

func (rh *raftHttpHandler)RemoveHandler(cid uint64) error {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	delete(rh.handlers, cid)
}

func (rh *raftHttpHandler)Destory() error {
	rh.mu.RLock()
	defer rh.mu.RUnlock()
	for _,_ = range rh.handlers {
		return os.ErrPermission
	}
	close(rh.stopc)
	select {
	case <-rh.donec:
	default:
	}
}

func (rh *raftHttpHandler)ServeHTTP(w http.ResponseWriter, r *http.Request) {
	 gcid, err := strconv.ParseUint(r.Header.Get("X-Etcd-Cluster-ID"), 10, 64)
	 rh.mu.RLock()
	 defer rh.mu.RUnlock()
	 if h, ok := rh.handlers(gcid); ok {
		 return h.ServeHTTP(w, r)
	 }
}

func newRaftHttpHandler(url *url.URL) (rh *raftHttpHandler, err error) {
	plog.Infof("start listen @ %v", url)

	rh = &raftHttpHandler{
		handlers: make(map[uint64]http.Handler),
		stopc: make(chan struct{}, 1),
		donec: make(chan struct{}, 1),
	}
	rh.ln, err = newStoppableListener(url.Host, rh.stopc)
	if err != nil {
		return nil, err
	}
	rh.svr = &http.Server{
		Handler: rh,
	}
	go func() {
		e := rh.svr.Serve(rh.ln)
		select {
		case <-rh.httpstopc:
		default:
			plog.Fatalf("Failed to serve rafthttp (%v)", err)
		}
		close(rh.donec)
	}()
	return rh, err
}

type holder struct {
	v interface{}
	c int64
	cf func(v interface{}) 
}

var objs map[string]*holder
var objMtx sync.RWMutex
var objOnce sync.Once

func lookup(k string) interface{} {
	objMtx.RLock()
	defer objMtx.RUnlock()
	if obj, ok := objs[k]; ok {
		obj.c = obj.c + 1
		return obj.v
	}
}

func add(k string, v interface{}, cf func(v interface{})) interface{} {
	tmp := &holder{
		v: v,
		c: 0,
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
	once.Do(func() {
		objs = make(map[string]*holder)
	})
	if r := lookup(url.Host); r != nil {
		return r.(*raftHttpHandler), nil
	}
	rh, err = newRaftHttpHandler(url)
	if err != nil {
		return nil, err
	}
	return add(url.Host, rh, func(v interface{}){
		v.(*raftHttpHandler).Destory()
	}).(*raftHttpHandler), nil
}

func ReleaseRaftHttpHandler(url *url.URL) {
	release(url.Host)
}