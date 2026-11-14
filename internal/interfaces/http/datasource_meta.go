package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/domain/port"
)

// 本文件实现数据源元数据读取与数据源相关的通用校验、加解密辅助。

// handleDataSourceMetadata 读取数据源元数据（取该数据源最近一次扫描的结果）。
func (s *Server) handleDataSourceMetadata(w http.ResponseWriter, r *http.Request) {
	ds, err := s.mustDataSource(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, terr := s.store.ListTasks(r.Context(), port.TaskFilter{DataSourceID: ds.ID, Page: 1, Limit: 1})
	if terr != nil {
		writeAPIError(w, errInternal("查询任务失败"))
		return
	}
	if len(tasks.Items) == 0 {
		writeDataMessage(w, map[string]any{
			"schemas": []port.SchemaMetaRow{}, "tables": []port.TableMetaRow{}, "columnCount": 0,
		}, "该数据源尚无扫描记录，元数据为空；请先发起扫描任务")
		return
	}
	task := tasks.Items[0]
	schemas, serr := s.store.ListSchemasByTask(r.Context(), task.ID)
	if serr != nil {
		writeAPIError(w, errInternal("读取模式元数据失败"))
		return
	}
	tables, terr := s.store.ListTablesByTask(r.Context(), task.ID)
	if terr != nil {
		writeAPIError(w, errInternal("读取表元数据失败"))
		return
	}
	columns, cerr := s.store.ListColumnsByTask(r.Context(), task.ID)
	if cerr != nil {
		writeAPIError(w, errInternal("读取字段元数据失败"))
		return
	}
	if schemas == nil {
		schemas = []port.SchemaMetaRow{}
	}
	if tables == nil {
		tables = []port.TableMetaRow{}
	}
	writeDataMessage(w, map[string]any{
		"schemas":     schemas,
		"tables":      tables,
		"columnCount": len(columns),
	}, fmt.Sprintf("元数据来自最近一次扫描任务（%s，%s）", task.ID, task.Status))
}

// mustDataSource 按路径参数读取数据源；不存在返回 1004。
func (s *Server) mustDataSource(w http.ResponseWriter, r *http.Request) (port.DataSource, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return port.DataSource{}, errInvalid("数据源标识不能为空")
	}
	ds, err := s.store.GetDataSource(r.Context(), id)
	if err != nil {
		return port.DataSource{}, errNotFound("指定的数据源不存在或已删除")
	}
	return ds, nil
}

// buildAddInput 校验并构造应用层入参。
func (s *Server) buildAddInput(name, dialect, host string, portNum int, service, username, password, dsn, include, excludeTables, excludeColumns string) (datasource.AddInput, error) {
	in := datasource.AddInput{
		Name: normalizeName(name), Dialect: strings.TrimSpace(dialect), Host: strings.TrimSpace(host),
		Port: portNum, ServiceName: strings.TrimSpace(service), Username: strings.TrimSpace(username),
		Password: password, DSN: strings.TrimSpace(dsn), IncludeSchemas: strings.TrimSpace(include),
		ExcludeTables: strings.TrimSpace(excludeTables), ExcludeColumns: strings.TrimSpace(excludeColumns),
	}
	if in.Name == "" {
		return in, errInvalid("实例标识不能为空")
	}
	if len(in.Name) < 2 || len(in.Name) > 64 {
		return in, errInvalid("实例标识长度必须为 2 到 64 个字符")
	}
	if in.Dialect == "" {
		return in, errInvalid("数据库类型不能为空")
	}
	if !dialectAllowed(in.Dialect) {
		return in, errInvalid("数据库类型不支持，可选值：" + dialectListText())
	}
	if in.Host == "" && in.DSN == "" {
		return in, errInvalid("主机与完整连接串至少填写一项")
	}
	if in.Port < 0 || in.Port > 65535 {
		return in, errInvalid("端口必须为 1 到 65535 之间的整数")
	}
	if in.Username == "" {
		return in, errInvalid("只读账号不能为空")
	}
	if in.Password == "" {
		return in, errInvalid("凭据不能为空")
	}
	return in, nil
}

// checkNameFree 校验实例标识未被占用（含软删除记录）。
func (s *Server) checkNameFree(r *http.Request, name string) error {
	existing, err := s.store.GetDataSourceByName(r.Context(), name)
	if err == nil && existing.ID != "" {
		if existing.DeletedAt != "" {
			return errInvalid("实例标识已存在（含已删除记录），请更换名称")
		}
		return errInvalid("实例标识已存在，请更换名称")
	}
	return nil
}

// applyRemarkAndEnabled 补写备注与启用状态（不改动凭据字段）。
func (s *Server) applyRemarkAndEnabled(ctx context.Context, id, remark string, enabled *bool) error {
	if remark == "" && enabled == nil {
		return nil
	}
	ds, err := s.store.GetDataSource(ctx, id)
	if err != nil {
		return err
	}
	if remark != "" {
		ds.Remark = remark
	}
	if enabled != nil {
		ds.Enabled = *enabled
	}
	return s.store.UpdateDataSource(ctx, ds)
}

// encryptPlain 加密明文；空串原样返回（与 datasource.Service 口径一致）。
func (s *Server) encryptPlain(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return s.cipher.Encrypt(plain, s.defaultAlg)
}

// updateLastTest 更新连接测试时间与结果。
func (s *Server) updateLastTest(ctx context.Context, id string, ok bool) {
	ds, err := s.store.GetDataSource(ctx, id)
	if err != nil {
		return
	}
	ds.LastTestAt = port.NowText()
	ds.LastTestOk = ok
	_ = s.store.UpdateDataSource(ctx, ds)
}

// dialectListText 返回可选数据库类型的中文说明。
func dialectListText() string {
	return "oracle（Oracle）、dameng（达梦）、mysql（MySQL）、postgres（PostgreSQL）、opengauss（openGauss）"
}
