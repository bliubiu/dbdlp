package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现规则、报告、仪表盘、审计日志与配置只读回显端点。

// handleListRules 规则清单：当前版本返回完整规则，指定历史版本返回快照摘要。
func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	if s.rulesLoader == nil {
		writeAPIError(w, errRuleLoad("规则加载器未初始化"))
		return
	}
	rules, currentVersion, err := s.rulesLoader()
	if err != nil {
		s.log(r.Context(), port.LevelERROR, "加载规则失败",
			port.Field{Key: "原因", Value: err.Error()})
		writeAPIError(w, errRuleLoad("规则文件校验失败："+err.Error()))
		return
	}
	if version == "" || version == currentVersion {
		writeData(w, map[string]any{
			"version": currentVersion,
			"rules":   newRuleViews(rules),
		})
		return
	}
	// 历史版本：快照表只落正则摘要，不落正则正文。
	snap, serr := s.store.ListRulesByVersion(r.Context(), version)
	if serr != nil {
		writeAPIError(w, errInternal("查询规则版本快照失败"))
		return
	}
	if len(snap) == 0 {
		writeAPIError(w, errNotFound("指定的规则版本快照不存在"))
		return
	}
	writeDataMessage(w, map[string]any{
		"version": version,
		"rules":   newRuleViews(snap),
	}, "历史规则版本仅保留正则摘要（regexDigest），正则正文不入库")
}

// handleListRuleSnapshots 规则版本快照列表。
func (s *Server) handleListRuleSnapshots(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	res, serr := s.store.ListRuleSnapshots(r.Context(), port.Pagination{Page: page, Limit: limit})
	if serr != nil {
		writeAPIError(w, errInternal("查询规则版本快照失败"))
		return
	}
	writeData(w, pageDataOf(res.Items, res.Total, res.Page, res.Limit, res.HasMore))
}

// createReportRequest 生成报告请求体。
type createReportRequest struct {
	TaskID  string   `json:"taskId"`
	Formats []string `json:"formats"`
}

// reportResultItem 单个格式的生成结果。
type reportResultItem struct {
	Format     string `json:"format"`
	ArtifactID string `json:"artifactId"`
	FilePath   string `json:"filePath"`
}

// handleCreateReport 生成报告：写 report_artifact 并记审计。
func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	var req createReportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		writeAPIError(w, errInvalid("任务标识不能为空"))
		return
	}
	if _, err := s.store.GetTask(r.Context(), taskID); err != nil {
		writeAPIError(w, errNotFound("指定的扫描任务不存在"))
		return
	}
	if len(req.Formats) == 0 {
		writeAPIError(w, errInvalid("报告格式不能为空，可选值：excel、html、json"))
		return
	}
	if s.reporterFactory == nil {
		writeAPIError(w, errInternal("报告器工厂未初始化"))
		return
	}
	bundle, berr := s.buildReportBundle(r, taskID)
	if berr != nil {
		writeError(w, berr)
		return
	}
	actor := currentUser(r.Context()).Username
	results := make([]reportResultItem, 0, len(req.Formats))
	for _, format := range req.Formats {
		f := strings.ToLower(strings.TrimSpace(format))
		switch port.ReportFormat(f) {
		case port.FormatExcel, port.FormatHTML, port.FormatJSON:
		default:
			s.audit(r.Context(), actor, actionReportExp, "scan_task", taskID,
				"生成报告失败：格式不支持 "+f, auditResultFailed, "", clientIP(r))
			writeAPIError(w, errInvalid("报告格式不支持，可选值：excel、html、json"))
			return
		}
		reporter, rerr := s.reporterFactory(port.ReportFormat(f))
		if rerr != nil {
			writeAPIError(w, errInternal("构建报告器失败："+rerr.Error()))
			return
		}
		path, eerr := reporter.Export(r.Context(), bundle, s.reportOutDir)
		if eerr != nil {
			s.audit(r.Context(), actor, actionReportExp, "scan_task", taskID,
				"生成报告失败："+f, auditResultFailed, eerr.Error(), clientIP(r))
			writeAPIError(w, errInternal("生成报告失败："+eerr.Error()))
			return
		}
		artifact := port.ReportArtifact{
			TaskID: taskID, Format: f, FilePath: path, GeneratedBy: actor,
			CreatedAt: port.NowText(), UpdatedAt: port.NowText(),
		}
		if info, serr := os.Stat(path); serr == nil {
			artifact.FileSize = info.Size()
		}
		if err := s.store.SaveReportArtifact(r.Context(), artifact); err != nil {
			writeAPIError(w, errInternal("保存报告产物记录失败"))
			return
		}
		results = append(results, reportResultItem{Format: f, ArtifactID: artifact.ID, FilePath: path})
	}
	s.audit(r.Context(), actor, actionReportExp, "scan_task", taskID,
		fmt.Sprintf("导出报告 %d 个格式", len(results)), auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "导出报告",
		port.Field{Key: "任务", Value: taskID},
		port.Field{Key: "格式数", Value: fmt.Sprintf("%d", len(results))},
		port.Field{Key: "操作者", Value: actor})
	writeData(w, results)
}

// handleListReports 报告列表。
func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	res, rerr := s.store.ListReports(r.Context(), taskID, port.Pagination{Page: page, Limit: limit})
	if rerr != nil {
		writeAPIError(w, errInternal("查询报告列表失败"))
		return
	}
	writeData(w, pageDataOf(res.Items, res.Total, res.Page, res.Limit, res.HasMore))
}

// handleDownloadReport 下载报告文件（记审计）。
func (s *Server) handleDownloadReport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, errInvalid("报告标识不能为空"))
		return
	}
	artifact, aerr := s.store.GetReportArtifact(r.Context(), id)
	if aerr != nil {
		writeAPIError(w, errNotFound("指定的报告产物不存在"))
		return
	}
	info, serr := os.Stat(artifact.FilePath)
	if serr != nil || info.IsDir() {
		s.audit(r.Context(), currentUser(r.Context()).Username, actionReportDown, "report_artifact", id,
			"下载报告失败：文件不存在", auditResultFailed, "", clientIP(r))
		writeAPIError(w, errNotFound("报告文件不存在或已被清理"))
		return
	}
	f, oerr := os.Open(artifact.FilePath)
	if oerr != nil {
		writeAPIError(w, errInternal("打开报告文件失败"))
		return
	}
	defer f.Close()
	actor := currentUser(r.Context()).Username
	s.audit(r.Context(), actor, actionReportDown, "report_artifact", id,
		fmt.Sprintf("下载报告 %s", artifact.Format), auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "下载报告",
		port.Field{Key: "报告ID", Value: id},
		port.Field{Key: "格式", Value: artifact.Format},
		port.Field{Key: "操作者", Value: actor})
	w.Header().Set("Content-Type", contentTypeByFormat(artifact.Format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName(artifact)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, artifact.FilePath, info.ModTime(), f)
}

// buildReportBundle 组装报告数据包。
func (s *Server) buildReportBundle(r *http.Request, taskID string) (port.ReportBundle, error) {
	t, terr := s.store.GetTask(r.Context(), taskID)
	if terr != nil {
		return port.ReportBundle{}, errNotFound("指定的扫描任务不存在")
	}
	findings, ferr := s.store.ListFindings(r.Context(), port.FindingFilter{TaskID: taskID, Page: 1, Limit: 200})
	if ferr != nil {
		return port.ReportBundle{}, errInternal("读取字段台账失败")
	}
	reviews, rerr := s.store.ListReviews(r.Context(), "", port.Pagination{Page: 1, Limit: 200})
	if rerr != nil {
		return port.ReportBundle{}, errInternal("读取复核记录失败")
	}
	checkpoints, cerr := s.store.ListPendingCheckpoints(r.Context(), taskID)
	if cerr != nil {
		return port.ReportBundle{}, errInternal("读取断点记录失败")
	}
	var rules []port.Rule
	if s.rulesLoader != nil {
		loaded, _, lerr := s.rulesLoader()
		if lerr != nil {
			return port.ReportBundle{}, errRuleLoad("加载规则失败：" + lerr.Error())
		}
		rules = loaded
	}
	return port.ReportBundle{
		Task: t, Findings: findings.Items, Rules: rules, Reviews: reviews.Items, Checkpoints: checkpoints,
	}, nil
}

// contentTypeByFormat 按报告格式给出下载内容类型。
func contentTypeByFormat(format string) string {
	switch format {
	case "excel":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "html":
		return "text/html; charset=utf-8"
	default:
		return "application/json; charset=utf-8"
	}
}

// downloadName 生成下载文件名。
func downloadName(a port.ReportArtifact) string {
	base := "report"
	if a.TaskID != "" {
		base = a.TaskID
	}
	ext := a.Format
	if ext == "" {
		ext = "json"
	}
	return filepath.Base(fmt.Sprintf("%s.%s", base, ext))
}
