package main

//
// start the master process, which is implemented
// in ../mr/master.go
//
// 运行master的方法：
// go run mrmaster.go pg*.txt
//
//
// Please do not change this file.
//

import "../mr"
import "time"
import "os"
import "fmt"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: mrmaster inputfiles...\n")
		os.Exit(1)
	}

	/*
		os.Arg[1:]表示split了的input文件
		10 代表 nReduce，也就是值 Reduce 任务的数量。
	 */
	m := mr.MakeMaster(os.Args[1:], 10)
	for m.Done() == false {
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second)
}
