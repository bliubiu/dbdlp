package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 本文件实现静态资源同端口托管：/api/v1/* 优先，未知路径回落到 index.html（SPA）。

// handleStatic 静态资源处理；静态目录不存在时统一返回 404。
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if !s.staticExists() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeAPIError(w, errNotFound("请求的资源不存在"))
		return
	}
	rel := strings.TrimPrefix(filepath.ToSlash(r.URL.Path), "/")
	if rel == "" {
		rel = "index.html"
	}
	target, ok := s.safeStaticPath(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		// SPA 回落：未知路径交由前端路由处理
		s.serveIndex(w, r)
		return
	}
	if info.IsDir() {
		index := filepath.Join(target, "index.html")
		if _, err := os.Stat(index); err != nil {
			s.serveIndex(w, r)
			return
		}
		http.ServeFile(w, r, index)
		return
	}
	http.ServeFile(w, r, target)
}

// serveIndex 输出 SPA 入口文件；入口不存在时返回 404。
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, index)
}

// safeStaticPath 解析静态资源路径，阻断目录穿越；越界返回 false。
func (s *Server) safeStaticPath(rel string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", false
	}
	target := filepath.Join(s.staticDir, cleaned)
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	root := s.staticDir
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", false
	}
	return abs, true
}
