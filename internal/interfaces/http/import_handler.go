package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"dbdlp/internal/application/datasource"
	"dbdlp/internal/domain/port"
)

// 本文件实现批量导入端点：multipart 上传 Excel/YAML，逐条门禁 + 部分成功，不整批回滚。
// 模板列定义见 docs/06 第 6.1 节；凭据列只给说明不预置真实值。

// importRow 单行导入数据（凭据仅内存瞬时）。
type importRow struct {
	Row            int
	Name           string
	Dialect        string
	Host           string
	Port           int
	ServiceName    string
	Username       string
	Password       string
	DSN            string
	IncludeSchemas string
	ExcludeTables  string
	ExcludeColumns string
}

// importError 单行失败明细（不含明文凭据）。
type importError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// importResult 批量导入结果（total = succeeded + failed 恒成立）。
type importResult struct {
	Total     int           `json:"total"`
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
	Errors    []importError `json:"errors"`
}

// importOutcome 单行导入结果。
type importOutcome struct {
	err string // 非空表示该行失败及中文原因
}

// 模板列顺序（Excel 表头与 YAML 字段一致）。
var templateColumns = []string{
	"实例标识", "数据库类型", "主机", "端口", "服务名/库名", "只读账号", "凭据",
	"完整连接串", "纳入模式", "排除表", "排除字段", "备注",
}

// 列别名 → 规范列名映射；兼容中文表头与英文字段名两种写法。
var columnAliases = map[string]string{
	"实例标识": "实例标识", "名称": "实例标识", "name": "实例标识",
	"数据库类型": "数据库类型", "类型": "数据库类型", "dialect": "数据库类型",
	"主机": "主机", "host": "主机",
	"端口": "端口", "port": "端口",
	"服务名/库名": "服务名/库名", "服务名": "服务名/库名", "servicename": "服务名/库名", "service_name": "服务名/库名",
	"只读账号": "只读账号", "账号": "只读账号", "username": "只读账号",
	"凭据": "凭据", "口令": "凭据", "password": "凭据",
	"完整连接串": "完整连接串", "连接串": "完整连接串", "dsn": "完整连接串",
	"纳入模式": "纳入模式", "includeschemas": "纳入模式", "include_schemas": "纳入模式",
	"排除表": "排除表", "excludetables": "排除表", "exclude_tables": "排除表",
	"排除字段": "排除字段", "excludecolumns": "排除字段", "exclude_columns": "排除字段",
	"备注": "备注", "remark": "备注",
}

// handleImportTemplate 下载批量导入模板（Excel / YAML）。
func (s *Server) handleImportTemplate(w http.ResponseWriter, r *http.Request) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	switch format {
	case "yaml", "yml":
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="datasources-template.yaml"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(yamlTemplateContent()))
	case "excel", "xlsx", "":
		content, err := buildExcelTemplate()
		if err != nil {
			writeAPIError(w, errInternal("生成 Excel 模板失败"))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="datasources-template.xlsx"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	default:
		writeAPIError(w, errInvalid("模板格式不支持，可选值：excel、yaml"))
	}
}

// handleImportDataSources 批量导入：逐条连接测试 + 逐条独立事务，允许部分成功。
func (s *Server) handleImportDataSources(w http.ResponseWriter, r *http.Request) {
	if s.dsSvc == nil {
		writeAPIError(w, errInternal("数据源服务未初始化"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
	if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeAPIError(w, errInvalid(fmt.Sprintf("上传文件超过 %d 字节上限，请拆分后重新上传", s.maxUploadBytes)))
			return
		}
		writeAPIError(w, errInvalid("上传内容无法解析，请使用 multipart/form-data 提交文件"))
		return
	}
	file, header, ferr := r.FormFile("file")
	if ferr != nil {
		writeAPIError(w, errInvalid("缺少上传文件，请使用 file 字段提交 Excel 或 YAML 文件"))
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var rows []importRow
	var perr error
	switch ext {
	case ".xlsx", ".xlsm":
		rows, perr = parseExcelRows(file)
	case ".yaml", ".yml":
		rows, perr = parseYAMLRows(file)
	default:
		writeAPIError(w, errInvalid("文件格式不支持，仅接受 .xlsx 或 .yaml/.yml 文件"))
		return
	}
	if perr != nil {
		writeError(w, perr)
		return
	}
	if len(rows) == 0 {
		writeAPIError(w, errInvalid("文件中没有可导入的数据行"))
		return
	}
	if len(rows) > s.maxImportRows {
		writeAPIError(w, errInvalid(fmt.Sprintf("文件超过 %d 行上限，请拆分后重新上传", s.maxImportRows)))
		return
	}

	result := s.importRows(r.Context(), rows)
	actor := currentUser(r.Context()).Username
	detail := fmt.Sprintf("批量导入：总数 %d、成功 %d、失败 %d", result.Total, result.Succeeded, result.Failed)
	s.audit(r.Context(), actor, actionDsImport, "data_source", "", detail, auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "批量导入完成",
		port.Field{Key: "总数", Value: strconv.Itoa(result.Total)},
		port.Field{Key: "成功", Value: strconv.Itoa(result.Succeeded)},
		port.Field{Key: "失败", Value: strconv.Itoa(result.Failed)},
		port.Field{Key: "操作者", Value: actor})

	message := fmt.Sprintf("批量导入完成：成功 %d 行、失败 %d 行，失败行未入库，可导出错误清单修正后重试",
		result.Succeeded, result.Failed)
	if result.Succeeded == 0 && result.Failed > 0 {
		// 全批失败：零写入，按 06 手册 6.3 口径返回 2001。
		writeJSON(w, http.StatusUnprocessableEntity, Resp{Code: CodeConnTestFail, Data: result, Message: message})
		return
	}
	writeJSON(w, http.StatusOK, Resp{Code: CodeOK, Data: result, Message: message})
}

// importRows 逐行执行「连接测试 + 落库」，每行独立事务，失败行不入库。
func (s *Server) importRows(ctx context.Context, rows []importRow) importResult {
	res := importResult{Total: len(rows), Errors: []importError{}}
	results := make([]importOutcome, len(rows))
	// 并发上限 2（对齐扫描并发低侵入约束），逐行独立落库。
	sem := make(chan struct{}, 2)
	done := make(chan struct{}, len(rows))
	for i := range rows {
		go func(i int) {
			sem <- struct{}{}
			defer func() { <-sem; done <- struct{}{} }()
			results[i] = s.importOneRow(ctx, rows[i])
		}(i)
	}
	for range rows {
		<-done
	}
	for i := range rows {
		if results[i].err != "" {
			res.Failed++
			res.Errors = append(res.Errors, importError{Row: rows[i].Row, Message: results[i].err})
			continue
		}
		res.Succeeded++
	}
	return res
}

// importOneRow 处理单行：字段校验 → 名称占用检查 → 连接测试 → 落库。
func (s *Server) importOneRow(ctx context.Context, row importRow) importOutcome {
	var out importOutcome
	in := datasource.AddInput{
		Name: row.Name, Dialect: row.Dialect, Host: row.Host, Port: row.Port,
		ServiceName: row.ServiceName, Username: row.Username, Password: row.Password, DSN: row.DSN,
		IncludeSchemas: row.IncludeSchemas, ExcludeTables: row.ExcludeTables, ExcludeColumns: row.ExcludeColumns,
	}
	if _, err := s.buildAddInput(in.Name, in.Dialect, in.Host, in.Port, in.ServiceName,
		in.Username, in.Password, in.DSN, in.IncludeSchemas, in.ExcludeTables, in.ExcludeColumns); err != nil {
		out.err = err.Error() + "；该行未入库"
		return out
	}
	existing, gerr := s.store.GetDataSourceByName(ctx, in.Name)
	if gerr == nil && existing.ID != "" {
		if existing.DeletedAt != "" {
			out.err = "实例标识已存在（含已删除记录），请更换名称；该行未入库"
			return out
		}
		// 幂等去重：同名有效记录已存在，跳过不重复写入（计入成功）。
		return out
	}
	if _, err := s.dsSvc.Add(ctx, in); err != nil {
		if strings.Contains(err.Error(), "连接测试未通过") {
			out.err = "连接测试未通过：" + err.Error() + "；该行未入库"
			return out
		}
		out.err = "写入失败：" + err.Error() + "；该行未入库"
		return out
	}
	return out
}
