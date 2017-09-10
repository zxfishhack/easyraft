@echo off
go get github.com/coreos/etcd

if not exist libs mkdir libs
if not exist bin  mkdir bin

go build -buildmode=c-archive -o libs\libraft.a .\main
gcc src\helpers.c libs\libraft.a -Ilibs  -o bin\libraft.dll -shared -lws2_32 -lwinmm
gcc tests\raft_test.c -L.\bin -llibraft -lws2_32 -lwinmm -o bin\test.exe