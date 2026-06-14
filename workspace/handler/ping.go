// ping.go - Ping 接口处理器
package handler

import (
	"net/http"

	"go-tiny-claw/response"
)

// Ping 处理 /ping 请求，返回 pong
func Ping(w http.ResponseWriter, r *http.Request) {
	response.Write(w, http.StatusOK, "pong", nil)
}
