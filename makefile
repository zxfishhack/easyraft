EASYRAFT_LIB:= libeasyraft.so
TESTS:= test

SRCS:= tests/main.cpp tests/raft_interfaces.cpp
OBJS:= $(subst .cpp,.o, $(SRCS))

CXX:= g++
CXXFLAGS:= -std=c++11 -fPIC
DEFINE:=
INCLUDE:= -Isrc -Iobjs -Iinclude
LDFLAGS:= -Llibs -leasyraft -pthread -Wl,-rpath=libs

PWD:= `pwd`

all: easyraft tests

.PHONY: easyraft
.PHONY: tests

.cpp.o:
	mkdir -p objs/tests
	@echo \# Compiling $<
	$(CXX) -c $(CXXFLAGS) $(DEFINE) $(INCLUDE) -oobjs/$@ $<

easyraft:
	mkdir -p libs objs bin
	@echo \# building $(EASYRAFT_LIB)
	go build -buildmode=c-archive -gccgoflags '-I$(PWD)/src' -o objs/libeasyraft.a ./main
	gcc src/easyraft.c objs/libeasyraft.a -Isrc -Iobjs -Iinclude -o libs/$(EASYRAFT_LIB) -shared -fPIC

tests: $(OBJS) 
	@echo $(OBJS)
	$(CXX) $(addprefix objs/, $(OBJS)) -o bin/$@ $(LDFLAGS)

clean:
	rm -rf objs bin libs