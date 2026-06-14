// main.go - 程序入口
package main

import (
	"fmt"
	"log"
	"net/http"

	"go-tiny-claw/handler"
)

func main() {
	http.HandleFunc("/ping", handler.Ping)

	fmt.Println("服务已启动，监听端口 :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
