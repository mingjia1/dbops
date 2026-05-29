package utils

import (
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

func ErrorResponse(ctx *gin.Context, code int, message string, err error) {
	errorInfo := &ErrorInfo{}
	if err != nil {
		errorInfo.Details = err.Error()
	}

	ctx.JSON(code, Response{
		Code:      code,
		Message:   message,
		Error:     errorInfo,
		Timestamp: time.Now(),
		TraceID:   ctx.GetString("trace_id"),
	})
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

func InternalServerErrorResponse(ctx *gin.Context, message string, err error) {
	ErrorResponse(ctx, 500, message, err)
}