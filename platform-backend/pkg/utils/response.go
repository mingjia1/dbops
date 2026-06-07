package utils

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	TraceID   string      `json:"trace_id,omitempty"`
}

type ErrorInfo struct {
	Type    string `json:"type"`
	Details string `json:"details"`
}

func SuccessResponse(ctx *gin.Context, data interface{}) {
	ctx.JSON(200, Response{
		Code:      200,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now(),
		TraceID:   ctx.GetString("trace_id"),
	})
}

// ErrorResponse P0: 之前 InternalServerErrorResponse 把 err.Error() 写进
// ErrorInfo.Details 直接返前端, 信息泄露 (SQL 表名/列名/内部路径) +
// UX 差 (用户看英文 SQL 噪音).
// 修: err 只在 server 端 log (含 trace_id), 前端拿到的是:
//   - 调用方传入的 message (业务友好)
//   - 通用 "internal error" (当 message 为空时)
//   - 永远带 trace_id, 用户报障时能定位 backend log.
func ErrorResponse(ctx *gin.Context, code int, message string, err error) {
	if err != nil {
		traceID := ctx.GetString("trace_id")
		log.Printf("[ERROR] trace_id=%s code=%d msg=%q err=%v", traceID, code, message, err)
	}

	errorInfo := &ErrorInfo{
		Type: httpCodeType(code),
	}
	// 5xx 不透 err (避免泄露内部), 4xx 可以透 (客户端错误, 帮助定位).
	if code < 500 && err != nil {
		errorInfo.Details = err.Error()
	}

	ctx.JSON(code, Response{
		Code:      code,
		Message:   defaultMessage(code, message),
		Error:     errorInfo,
		Timestamp: time.Now(),
		TraceID:   ctx.GetString("trace_id"),
	})
}

// defaultMessage 5xx 时不暴露 err, 统一返 "Internal error, contact support with trace_id".
func defaultMessage(code int, override string) string {
	if override != "" {
		return override
	}
	if code >= 500 {
		return "Internal error"
	}
	if code == 404 {
		return "Not found"
	}
	if code == 401 {
		return "Unauthorized"
	}
	if code == 403 {
		return "Forbidden"
	}
	if code == 400 {
		return "Bad request"
	}
	return "Error"
}

func httpCodeType(code int) string {
	switch {
	case code >= 500:
		return "internal"
	case code == 404:
		return "not_found"
	case code == 401:
		return "unauthorized"
	case code == 403:
		return "forbidden"
	case code == 400:
		return "bad_request"
	default:
		return "error"
	}
}

func BadRequestResponse(ctx *gin.Context, message string) {
	ErrorResponse(ctx, 400, message, nil)
}

func UnauthorizedResponse(ctx *gin.Context, message string) {
	ErrorResponse(ctx, 401, message, nil)
}

func ForbiddenResponse(ctx *gin.Context, message string) {
	ErrorResponse(ctx, 403, message, nil)
}

func NotFoundResponse(ctx *gin.Context, message string) {
	ErrorResponse(ctx, 404, message, nil)
}

// InternalServerErrorResponse 5xx 不再透 err.Error() (走 log + 通用 message).
func InternalServerErrorResponse(ctx *gin.Context, message string, err error) {
	ErrorResponse(ctx, 500, message, err)
}