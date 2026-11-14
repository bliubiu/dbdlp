// Package httpapi dbdlp 网页控制台 HTTP API 层。
// 分层约定：本包只做协议适配（解析参数、校验入参、组装响应、调用用例服务），
// 业务规则一律下沉到 application 层，本包不直连数据库、不复用第三方 HTTP 框架。
package httpapi

import (
	"encoding/json"
	"net/http"
)

// 业务错误码（与 docs/02 §13.3 错误码规范一致）。
const (
	CodeOK           = 0    // 成功
	CodeInvalidParam = 1001 // 参数校验失败
	CodeUnauthorized = 1002 // 认证失败／会话过期
	CodeForbidden    = 1003 // 无权限
	CodeNotFound     = 1004 // 资源不存在
	CodeConnTestFail = 2001 // 连接测试失败
	CodeRuleLoadFail = 2002 // 规则加载失败
	CodeKeyUnusable  = 2003 // 密钥不可用
	CodeTaskConflict = 3001 // 任务状态冲突
	CodeStorageFail  = 3002 // 存储写入失败
	CodeInternal     = 5000 // 未分类内部错误
)

// Resp 统一响应体：{"code":0,"data":{},"message":""}。
type Resp struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// PageData 分页数据：{"items":[],"total":0,"page":1,"limit":20,"hasMore":false}。
type PageData struct {
	Items   any   `json:"items"`
	Total   int64 `json:"total"`
	Page    int   `json:"page"`
	Limit   int   `json:"limit"`
	HasMore bool  `json:"hasMore"`
}

// apiError 带业务错误码与 HTTP 状态的内部错误类型。
type apiError struct {
	code int
	http int
	msg  string
}

// Error 实现 error 接口。
func (e *apiError) Error() string { return e.msg }

// newAPIError 构造带错误码与 HTTP 状态的错误。
func newAPIError(code, httpStatus int, msg string) *apiError {
	return &apiError{code: code, http: httpStatus, msg: msg}
}

// errInvalid 参数校验失败（400／1001）。
func errInvalid(msg string) *apiError {
	return newAPIError(CodeInvalidParam, http.StatusBadRequest, msg)
}

// errUnauthorized 认证失败或会话过期（401／1002）。
func errUnauthorized(msg string) *apiError {
	return newAPIError(CodeUnauthorized, http.StatusUnauthorized, msg)
}

// errForbidden 无权限（403／1003）。
func errForbidden(msg string) *apiError { return newAPIError(CodeForbidden, http.StatusForbidden, msg) }

// errNotFound 资源不存在（404／1004）。
func errNotFound(msg string) *apiError { return newAPIError(CodeNotFound, http.StatusNotFound, msg) }

// errConnTest 连接测试失败（422／2001）。
func errConnTest(msg string) *apiError {
	return newAPIError(CodeConnTestFail, http.StatusUnprocessableEntity, msg)
}

// errRuleLoad 规则加载失败（422／2002）。
func errRuleLoad(msg string) *apiError {
	return newAPIError(CodeRuleLoadFail, http.StatusUnprocessableEntity, msg)
}

// errKeyUnusable 密钥不可用（500／2003）。
func errKeyUnusable(msg string) *apiError {
	return newAPIError(CodeKeyUnusable, http.StatusInternalServerError, msg)
}

// errTaskConflict 任务状态冲突（409／3001）。
func errTaskConflict(msg string) *apiError {
	return newAPIError(CodeTaskConflict, http.StatusConflict, msg)
}

// errStorage 存储写入失败（500／3002）。
func errStorage(msg string) *apiError {
	return newAPIError(CodeStorageFail, http.StatusInternalServerError, msg)
}

// errInternal 未分类内部错误（500／5000）。
func errInternal(msg string) *apiError {
	return newAPIError(CodeInternal, http.StatusInternalServerError, msg)
}

// writeJSON 输出统一响应体。
func writeJSON(w http.ResponseWriter, httpStatus int, body Resp) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(body)
}

// writeData 输出成功响应（code=0）。
func writeData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Resp{Code: CodeOK, Data: data, Message: ""})
}

// writeDataMessage 输出带中文说明的成功响应（用于需要提示调用方的降级或注意事项）。
func writeDataMessage(w http.ResponseWriter, data any, message string) {
	writeJSON(w, http.StatusOK, Resp{Code: CodeOK, Data: data, Message: message})
}

// writeError 输出错误响应；未知错误一律降级为 5000，不回显内部细节。
func writeError(w http.ResponseWriter, err error) {
	if e, ok := err.(*apiError); ok {
		writeJSON(w, e.http, Resp{Code: e.code, Data: map[string]any{}, Message: e.msg})
		return
	}
	writeJSON(w, http.StatusInternalServerError, Resp{
		Code: CodeInternal, Data: map[string]any{}, Message: "系统内部错误，请联系管理员",
	})
}

// writeAPIError 输出指定业务错误。
func writeAPIError(w http.ResponseWriter, err *apiError) {
	writeJSON(w, err.http, Resp{Code: err.code, Data: map[string]any{}, Message: err.msg})
}
