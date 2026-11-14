package httpapi

import (
	"net/http"
	"strings"

	"dbdlp/internal/domain/port"
)

// 本文件实现结果台账与人工复核端点。

// 复核结论枚举（与 04 文档 P08 三结论一致）。
const (
	reviewConfirmed     = "confirmed"
	reviewFalsePositive = "false_positive"
	reviewPending       = "pending"
)

// createReviewRequest 提交复核请求体。
type createReviewRequest struct {
	FindingID  string `json:"findingId"`
	Conclusion string `json:"conclusion"`
	Comment    string `json:"comment"`
}

// handleListFindings 字段台账列表（服务端分页）。
func (s *Server) handleListFindings(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	f := port.FindingFilter{
		TaskID:       strings.TrimSpace(q.Get("taskId")),
		RiskLevel:    port.RiskLevel(strings.TrimSpace(q.Get("riskLevel"))),
		ReviewStatus: strings.TrimSpace(q.Get("reviewStatus")),
		Keyword:      strings.TrimSpace(q.Get("keyword")),
		Page:         page, Limit: limit,
		Sort:  strings.TrimSpace(q.Get("sort")),
		Order: strings.TrimSpace(q.Get("order")),
	}
	res, serr := s.store.ListFindings(r.Context(), f)
	if serr != nil {
		writeAPIError(w, errInternal("查询字段台账失败"))
		return
	}
	writeData(w, pageDataOf(newFindingViews(res.Items), res.Total, res.Page, res.Limit, res.HasMore))
}

// handleGetFinding 字段台账详情（含掩码样例）。
func (s *Server) handleGetFinding(w http.ResponseWriter, r *http.Request) {
	f, err := s.mustFinding(r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, newFindingView(f))
}

// handleListHits 字段命中明细（仅掩码值）。
func (s *Server) handleListHits(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, errInvalid("字段台账标识不能为空"))
		return
	}
	if _, err := s.store.GetFinding(r.Context(), id); err != nil {
		writeAPIError(w, errNotFound("指定的字段台账不存在"))
		return
	}
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	res, herr := s.store.ListHits(r.Context(), id, port.Pagination{Page: page, Limit: limit})
	if herr != nil {
		writeAPIError(w, errInternal("查询命中明细失败"))
		return
	}
	writeData(w, pageDataOf(res.Items, res.Total, res.Page, res.Limit, res.HasMore))
}

// handleCreateReview 提交人工复核结论（不覆盖机器原始结论）。
func (s *Server) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	var req createReviewRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	findingID := strings.TrimSpace(req.FindingID)
	if findingID == "" {
		writeAPIError(w, errInvalid("字段台账标识不能为空"))
		return
	}
	conclusion := strings.TrimSpace(req.Conclusion)
	switch conclusion {
	case reviewConfirmed, reviewFalsePositive, reviewPending:
	default:
		writeAPIError(w, errInvalid("复核结论非法，可选值：confirmed（确认敏感）、false_positive（误报）、pending（待确认）"))
		return
	}
	if _, err := s.store.GetFinding(r.Context(), findingID); err != nil {
		writeAPIError(w, errNotFound("指定的字段台账不存在"))
		return
	}
	user := currentUser(r.Context())
	rec := port.ReviewRecord{
		FindingID: findingID, Reviewer: user.Username, Conclusion: conclusion,
		Comment: strings.TrimSpace(req.Comment), ReviewedAt: port.NowText(),
		CreatedAt: port.NowText(), UpdatedAt: port.NowText(),
	}
	if err := s.store.SaveReview(r.Context(), rec); err != nil {
		s.audit(r.Context(), user.Username, actionReview, "finding_column", findingID,
			"提交复核结论失败", auditResultFailed, err.Error(), clientIP(r))
		writeAPIError(w, errInternal("提交复核结论失败，请稍后重试"))
		return
	}
	if err := s.store.UpdateFindingReviewStatus(r.Context(), findingID, conclusion); err != nil {
		s.log(r.Context(), port.LevelWARN, "更新字段复核状态失败",
			port.Field{Key: "字段台账ID", Value: findingID},
			port.Field{Key: "原因", Value: err.Error()})
	}
	s.audit(r.Context(), user.Username, actionReview, "finding_column", findingID,
		"提交复核结论："+conclusion, auditResultOK, "", clientIP(r))
	s.log(r.Context(), port.LevelINFO, "提交复核结论",
		port.Field{Key: "字段台账ID", Value: findingID},
		port.Field{Key: "结论", Value: conclusion},
		port.Field{Key: "操作者", Value: user.Username})
	writeData(w, map[string]any{"id": rec.ID})
}

// handleListReviews 复核历史。
func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePageLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	findingID := strings.TrimSpace(r.URL.Query().Get("findingId"))
	res, rerr := s.store.ListReviews(r.Context(), findingID, port.Pagination{Page: page, Limit: limit})
	if rerr != nil {
		writeAPIError(w, errInternal("查询复核记录失败"))
		return
	}
	writeData(w, pageDataOf(res.Items, res.Total, res.Page, res.Limit, res.HasMore))
}

// mustFinding 按路径参数读取字段台账；不存在返回 1004。
func (s *Server) mustFinding(r *http.Request) (port.ColumnFinding, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return port.ColumnFinding{}, errInvalid("字段台账标识不能为空")
	}
	f, err := s.store.GetFinding(r.Context(), id)
	if err != nil {
		return port.ColumnFinding{}, errNotFound("指定的字段台账不存在")
	}
	return f, nil
}
