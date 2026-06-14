package response

import (
	"encoding/json"
	"net/http"
)

// JSON 统一响应结构
type JSON struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Write 写入统一 JSON 响应
func Write(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(JSON{
		Code:    statusCode,
		Message: message,
		Data:    data,
	})
}

// Success 快捷写入成功响应
func Success(w http.ResponseWriter, data interface{}) {
	Write(w, http.StatusOK, "操作成功", data)
}

// Error 快捷写入失败响应（message 必须使用中文）
func Error(w http.ResponseWriter, statusCode int, message string) {
	Write(w, statusCode, message, nil)
}
