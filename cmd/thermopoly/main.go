// thermopoly 是药物晶型转变热分析判读服务入口。
// 用法：
//
//	thermopoly [--addr :8080] [--db /path/thermopoly.db] [--smoke-test]
//
// --smoke-test 不启动长驻服务，而是真实创建试验、导入曲线、执行
// 基线校正/峰检测/事件判读/快照发布，关闭并重开数据库验证持久化，
// 全部通过后以 0 退出（Docker 双架构验证的唯一判据）。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"task233-thermopoly/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	db := flag.String("db", "", "SQLite database file path (empty = in-memory)")
	smoke := flag.Bool("smoke-test", false, "run end-to-end self test and exit")
	flag.Parse()

	if *smoke {
		if err := server.SmokeTest(*db); err != nil {
			log.Printf("smoke-test FAILED: %v", err)
			os.Exit(1)
		}
		return
	}

	srv, err := server.New(server.Config{Addr: *addr, DB: *db})
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("server exited: %v", err)
		os.Exit(1)
	}
}
