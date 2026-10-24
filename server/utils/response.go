package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// 错误码常量
const (
	CodeSuccess       = 0
	CodeBadRequest    = 1
	CodeUnauthorized  = 2
	CodeForbidden     = 3
	CodeNotFound      = 4
	CodeInternalError = 5
	CodeConflict      = 6 // 版本冲突（HTTP 409）：保存时 base_version 落后于服务端
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

func SuccessMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, CodeInternalError, message)
}

// Conflict 版本冲突（HTTP 409）：data 里带三方内容（base / server / mine）供前端并排对比
func Conflict(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusConflict, Response{
		Code:    CodeConflict,
		Message: message,
		Data:    data,
	})
}
