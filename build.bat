@echo off

if not exist libs mkdir libs
if not exist bin  mkdir bin
if not exist objs mkdir objs

go build -buildmode=c-archive -o objs\libeasyraft.a .\main
gcc src\easyraft.c objs\libeasyraft.a -Iobjs -Isrc -Iinclude  -o bin\easyraft.dll -shared -lws2_32 -lwinmm
dlltool --def src\easyraft.def --dllname easyraft.dll --output-lib libs\easyraft.lib