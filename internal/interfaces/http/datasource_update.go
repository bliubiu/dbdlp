package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现数据源修改端点：含凭据变更时必须重连测试通过才保存。

// dataSourcePatchRequest 数据源修改请求体（指针表示是否提交该字段）。
type dataSourcePatchRequest struct {
	Name           *string `json:"name"`
	Dialect        *string `json:"dialect"`
	Host           *string `json:"host"`
	Port           *int    `json:"port"`
	ServiceName    *string `json:"serviceName"`
	Username       *string `json:"username"`
	Password       *string `json:"password"`
	DSN            *string `json:"dsn"`
	IncludeSchemas *string `json:"includeSchemas"`
	ExcludeTables  *string `json:"excludeTables"`
	ExcludeColumns *string `json:"excludeColumns"`
	Enabled        *bool   `json:"enabled"`
	Remark         *string `json:"remark"`
}

// handleUpdateDataSource 修改数据源：含凭据变更时必须重连测试通过才保存。
func (s *Server) handleUpdateDataSource(w http.ResponseWriter, r *http.Request) {
	if s.dsSvc == nil {
		writeAPIError(w, errInternal("数据源服务未初始化"))
		return
	}
	ds, err := s.mustDataSource(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req dataSourcePatchRequest
	if derr := decodeJSON(w, r, &req); derr != nil {
		writeError(w, derr)
		return
	}
	actor := currentUser(r.Context()).Username

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

	newName := ds.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		newName = normalizeName(*req.Name)
		if newName != ds.Name {
			if err := s.checkNameFree(r, newName); err != nil {
				writeError(w, err)
				return
			}
		}
	}
	newDialect := string(ds.Dialect)
	if req.Dialect != nil && strings.TrimSpace(*req.Dialect) != "" {
		newDialect = strings.TrimSpace(*req.Dialect)
		if !dialectAllowed(newDialect) {
			writeAPIError(w, errInvalid("数据库类型不支持，可选值："+dialectListText()))
			return
		}
	}
	newHost := host
	if req.Host != nil && strings.TrimSpace(*req.Host) != "" {
		newHost = strings.TrimSpace(*req.Host)
	}
	newPort := ds.Port
	if req.Port != nil {
		if *req.Port < 1 || *req.Port > 65535 {
			writeAPIError(w, errInvalid("端口必须为 1 到 65535 之间的整数"))
			return
		}
		newPort = *req.Port
	}
	newService := ds.ServiceName
	if req.ServiceName != nil {
		newService = strings.TrimSpace(*req.ServiceName)
	}
	newUser := ds.Username
	if req.Username != nil && strings.TrimSpace(*req.Username) != "" {
		newUser = strings.TrimSpace(*req.Username)
	}
	newPass := pass
	if req.Password != nil && *req.Password != "" {
		newPass = *req.Password
	}
	newDSN := dsn
	if req.DSN != nil {
		newDSN = strings.TrimSpace(*req.DSN)
	}
	newInclude := ds.IncludeSchemas
	if req.IncludeSchemas != nil {
		newInclude = strings.TrimSpace(*req.IncludeSchemas)
	}
	newExcludeTables := ds.ExcludeTables
	if req.ExcludeTables != nil {
		newExcludeTables = strings.TrimSpace(*req.ExcludeTables)
	}
	newExcludeColumns := ds.ExcludeColumns
	if req.ExcludeColumns != nil {
		newExcludeColumns = strings.TrimSpace(*req.ExcludeColumns)
	}
	newEnabled := ds.Enabled
	if req.Enabled != nil {
		newEnabled = *req.Enabled
	}
	newRemark := ds.Remark
	if req.Remark != nil {
		newRemark = strings.TrimSpace(*req.Remark)
	}

	// 凭据相关字段变更判定：主机、端口、服务名、账号、口令、连接串
	credentialChanged := newHost != host || newPort != ds.Port || newService != ds.ServiceName ||
		newUser != ds.Username || newPass != pass || newDSN != dsn
	if credentialChanged {
		cfg := port.DataSourceConfig{
			Name: newName, Dialect: port.Dialect(newDialect), Host: newHost, Port: newPort,
			ServiceName: newService, Username: newUser, Password: newPass, DSN: newDSN,
		}
		if terr := s.dsSvc.Test(r.Context(), cfg); terr != nil {
			s.audit(r.Context(), actor, actionDsUpdate, "data_source", ds.ID,
				"修改数据源凭据后连接测试未通过，已保持原值", auditResultFailed, terr.Error(), clientIP(r))
			s.log(r.Context(), port.LevelWARN, "数据源凭据变更后连接测试未通过，已保持原值",
				port.Field{Key: "数据源ID", Value: ds.ID},
				port.Field{Key: "原因", Value: terr.Error()})
			writeAPIError(w, errConnTest("连接测试未通过："+terr.Error()+"，配置未保存"))
			return
		}
	}

	updated := ds
	updated.Name = newName
	updated.Dialect = port.Dialect(newDialect)
	updated.Port = newPort
	updated.ServiceName = newService
	updated.Username = newUser
	updated.IncludeSchemas = newInclude
	updated.ExcludeTables = newExcludeTables
	updated.ExcludeColumns = newExcludeColumns
	updated.Enabled = newEnabled
	updated.Remark = newRemark
	if credentialChanged {
		hostEnc, eerr := s.encryptPlain(newHost)
		if eerr != nil {
			writeAPIError(w, errKeyUnusable("加密主机失败，密钥不可用"))
			return
		}
		passEnc, eerr := s.encryptPlain(newPass)
		if eerr != nil {
			writeAPIError(w, errKeyUnusable("加密口令失败，密钥不可用"))
			return
		}
		dsnEnc, eerr := s.encryptPlain(newDSN)
		if eerr != nil {
			writeAPIError(w, errKeyUnusable("加密连接串失败，密钥不可用"))
			return
		}
		updated.HostEnc = hostEnc
		updated.PasswordEnc = passEnc
		updated.DSNEnc = dsnEnc
		updated.LastTestAt = port.NowText()
		updated.LastTestOk = true
	}
	if err := s.store.UpdateDataSource(r.Context(), updated); err != nil {
		s.audit(r.Context(), actor, actionDsUpdate, "data_source", ds.ID, "修改数据源失败", auditResultFailed, err.Error(), clientIP(r))
		writeAPIError(w, errInternal("保存数据源失败，请稍后重试"))
		return
	}
	s.audit(r.Context(), actor, actionDsUpdate, "data_source", ds.ID,
		fmt.Sprintf("修改数据源 %s（凭据变更：%t）", newName, credentialChanged), auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "修改数据源成功",
		port.Field{Key: "数据源ID", Value: ds.ID},
		port.Field{Key: "凭据变更", Value: fmt.Sprintf("%t", credentialChanged)},
		port.Field{Key: "操作者", Value: actor})
	writeData(w, map[string]any{"id": ds.ID})
}
