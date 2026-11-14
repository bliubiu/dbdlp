package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现数据源相关端点：列表、新增、详情、修改、删除、连接测试、元数据读取。
// 所有写操作均走 datasource.Service（含「连接测试通过才保存」门禁）。

// dataSourceRequest 数据源新增请求体。
type dataSourceRequest struct {
	Name           string `json:"name"`
	Dialect        string `json:"dialect"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	ServiceName    string `json:"serviceName"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	DSN            string `json:"dsn"`
	IncludeSchemas string `json:"includeSchemas"`
	ExcludeTables  string `json:"excludeTables"`
	ExcludeColumns string `json:"excludeColumns"`
	Enabled        *bool  `json:"enabled"`
	Remark         string `json:"remark"`
}

// handleListDataSources 数据源列表（服务端分页）。
func (s *Server) handleListDataSources(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	f := port.DataSourceFilter{
		Dialect: port.Dialect(strings.TrimSpace(q.Get("dialect"))),
		Keyword: strings.TrimSpace(q.Get("keyword")),
		Page:    page, Limit: limit,
	}
	res, serr := s.store.ListDataSources(r.Context(), f)
	if serr != nil {
		s.log(r.Context(), port.LevelERROR, "查询数据源列表失败",
			port.Field{Key: "原因", Value: serr.Error()})
		writeAPIError(w, errInternal("查询数据源列表失败"))
		return
	}
	writeData(w, pageDataOf(s.newDataSourceViews(res.Items), res.Total, res.Page, res.Limit, res.HasMore))
}

// handleGetDataSource 数据源详情（密码恒为占位串）。
func (s *Server) handleGetDataSource(w http.ResponseWriter, r *http.Request) {
	ds, err := s.mustDataSource(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, s.newDataSourceView(ds))
}

// handleCreateDataSource 新增数据源：先连接测试，通过才保存。
func (s *Server) handleCreateDataSource(w http.ResponseWriter, r *http.Request) {
	if s.dsSvc == nil {
		writeAPIError(w, errInternal("数据源服务未初始化"))
		return
	}
	var req dataSourceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	in, err := s.buildAddInput(req.Name, req.Dialect, req.Host, req.Port, req.ServiceName,
		req.Username, req.Password, req.DSN, req.IncludeSchemas, req.ExcludeTables, req.ExcludeColumns)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.checkNameFree(r, in.Name); err != nil {
		writeError(w, err)
		return
	}
	actor := currentUser(r.Context()).Username
	id, aerr := s.dsSvc.Add(r.Context(), in)
	if aerr != nil {
		s.log(r.Context(), port.LevelWARN, "新增数据源失败",
			port.Field{Key: "数据源", Value: in.Name},
			port.Field{Key: "原因", Value: aerr.Error()})
		s.audit(r.Context(), actor, actionDsCreate, "data_source", in.Name, "新增数据源失败", auditResultFailed, aerr.Error(), clientIP(r))
		if strings.Contains(aerr.Error(), "连接测试未通过") {
			writeAPIError(w, errConnTest(aerr.Error()+"，配置未保存"))
			return
		}
		writeAPIError(w, errInternal("新增数据源失败，请稍后重试"))
		return
	}
	// 备注与启用状态由应用层服务统一落库，此处按请求补齐（不触碰凭据字段）。
	if err := s.applyRemarkAndEnabled(r.Context(), id, req.Remark, req.Enabled); err != nil {
		s.log(r.Context(), port.LevelWARN, "数据源备注或启用状态补写失败",
			port.Field{Key: "数据源ID", Value: id},
			port.Field{Key: "原因", Value: err.Error()})
	}
	s.audit(r.Context(), actor, actionDsCreate, "data_source", id, "新增数据源 "+in.Name, auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "新增数据源成功",
		port.Field{Key: "数据源", Value: in.Name},
		port.Field{Key: "数据源ID", Value: id},
		port.Field{Key: "操作者", Value: actor})
	writeData(w, map[string]any{"id": id, "lastTestOk": true})
}

// handleDeleteDataSource 删除数据源（软删）：运行中有任务时拒绝。
func (s *Server) handleDeleteDataSource(w http.ResponseWriter, r *http.Request) {
	ds, err := s.mustDataSource(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	running, cerr := s.store.CountRunningTasksBySource(r.Context(), ds.ID)
	if cerr != nil {
		writeAPIError(w, errInternal("校验运行中任务失败"))
		return
	}
	if running > 0 {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionDsDelete, "data_source", ds.ID,
			"删除被拒绝：存在运行中的任务", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errTaskConflict(fmt.Sprintf("该数据源存在 %d 个运行中的任务，请先停止任务再删除", running)))
		return
	}
	if err := s.store.DeleteDataSource(r.Context(), ds.ID); err != nil {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionDsDelete, "data_source", ds.ID,
			"删除数据源失败", auditResultFailed, err.Error(), clientIP(r))
		writeAPIError(w, errInternal("删除数据源失败，请稍后重试"))
		return
	}
	s.audit(r.Context(), currentUser(r.Context()).Username, actionDsDelete, "data_source", ds.ID,
		"删除数据源 "+ds.Name, auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "删除数据源成功",
		port.Field{Key: "数据源", Value: ds.Name},
		port.Field{Key: "数据源ID", Value: ds.ID})
	writeData(w, map[string]any{})
}

// handleTestDataSource 连接测试：只读探针，不改连接配置。
func (s *Server) handleTestDataSource(w http.ResponseWriter, r *http.Request) {
	if s.dsSvc == nil {
		writeAPIError(w, errInternal("数据源服务未初始化"))
		return
	}
	ds, err := s.mustDataSource(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	host, herr := s.cipher.Decrypt(ds.HostEnc)
	if herr != nil {
		writeAPIError(w, errKeyUnusable("解密主机失败，密钥不可用"))
		return
	}
	pass, herr := s.cipher.Decrypt(ds.PasswordEnc)
	if herr != nil {
		writeAPIError(w, errKeyUnusable("解密口令失败，密钥不可用"))
		return
	}
	dsn, herr := s.cipher.Decrypt(ds.DSNEnc)
	if herr != nil {
		writeAPIError(w, errKeyUnusable("解密连接串失败，密钥不可用"))
		return
	}
	cfg := port.DataSourceConfig{
		ID: ds.ID, Name: ds.Name, Dialect: ds.Dialect, Host: host, Port: ds.Port,
		ServiceName: ds.ServiceName, Username: ds.Username, Password: pass, DSN: dsn,
		MaxOpenConns: ds.ConnMaxOpen, MaxIdleConns: ds.ConnMaxIdle,
	}
	start := nowMillis()
	terr := s.dsSvc.Test(r.Context(), cfg)
	latency := nowMillis() - start
	actor := currentUser(r.Context()).Username
	if terr != nil {
		s.audit(r.Context(), actor, actionDsTest, "data_source", ds.ID,
			"连接测试未通过", auditResultFailed, terr.Error(), clientIP(r))
		s.updateLastTest(r.Context(), ds.ID, false)
		writeAPIError(w, errConnTest("连接测试未通过："+terr.Error()))
		return
	}
	s.updateLastTest(r.Context(), ds.ID, true)
	s.audit(r.Context(), actor, actionDsTest, "data_source", ds.ID,
		fmt.Sprintf("连接测试通过（%d 毫秒）", latency), auditResultOK, "", clientIP(r))
	writeData(w, map[string]any{
		"ok":        true,
		"latencyMs": latency,
		"message":   "连接测试通过",
	})
}
