package main

import (
	"fmt"
	"github.com/coreos/etcd/wal"
	"os"
)

func main() {
	if len(os.Args)!= 2 {
		fmt.Printf("usage: %v <wal dir>\n", os.Args[0])
		return
	}
	ret := wal.Repair(os.Args[1])
	fmt.Printf("repair wal return %v\n", ret)
}	
