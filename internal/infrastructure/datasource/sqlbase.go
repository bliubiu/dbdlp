// Package datasource 多数据源驱动适配器（Oracle / 达梦 / 测试用 SQLite）。
// 统一只读连接、只读守卫、主键范围分页采样；上层只依赖 port.Driver。
package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	// 注册数据库驱动：go-ora(v2)→"oracle"，达梦→"dm"，modernc→"sqlite"。
	"dbdlp/internal/domain/port"
	_ "gitee.com/chunanyong/dm"
	_ "github.com/sijms/go-ora/v2"
	_ "modernc.org/sqlite"
)

// roConn 只读连接封装（database/sql）。
type roConn struct {
	db *sql.DB
}

func (c *roConn) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }

// Query 执行只读查询；入口处白名单校验，拒绝任何写/DDL/多语句/注释绕过。
func (c *roConn) Query(ctx context.Context, q string, args ...any) (port.Rows, error) {
	if err := guardReadOnly(q); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("只读查询失败：%w", err)
	}
	return rows, nil
}

func (c *roConn) Close() error { return c.db.Close() }

// guardReadOnly 只读守卫（docs/02 §5.2）。
func guardReadOnly(q string) error {
	s := strings.TrimSpace(q)
	if s == "" {
		return fmt.Errorf("空语句被拒绝")
	}
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") && !strings.HasPrefix(upper, "PRAGMA") {
		return fmt.Errorf("仅允许 SELECT/WITH/PRAGMA 只读语句：%s", truncate(q, 32))
	}
	up := stripComments(s)
	for _, kw := range []string{"INSERT", "UPDATE", "DELETE", "MERGE", "DROP", "ALTER", "CREATE", "TRUNCATE", "CALL", "EXEC"} {
		if containsWord(up, kw) {
			return fmt.Errorf("检测到写/DDL 关键字，只读连接已拒绝：%s", kw)
		}
	}
	if strings.ContainsRune(s, ';') {
		return fmt.Errorf("禁止多语句批处理（含分号）")
	}
	return nil
}

func stripComments(s string) string {
	out := make([]rune, 0, len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if i+1 < len(runes) && runes[i] == '-' && runes[i+1] == '-' {
			for i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*' {
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		out = append(out, runes[i])
	}
	return string(out)
}

func containsWord(s, word string) bool {
	upper := strings.ToUpper(s)
	w := strings.ToUpper(word)
	for i := 0; i+len(w) <= len(upper); i++ {
		if upper[i:i+len(w)] == w {
			before := i == 0 || !isIdent(upper[i-1])
			after := i+len(w) >= len(upper) || !isIdent(upper[i+len(w)])
			if before && after {
				return true
			}
		}
	}
	return false
}

func isIdent(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// quoteIdent 通用标识符引用（各方言可覆写）。
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// scanToStrings 将结果集读为字符串矩阵，文本字段截断至 maxTextLen，二进制为空串。
func scanToStrings(rows port.Rows, maxTextLen int) (cols []string, vals [][]string, err error) {
	cols, err = rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		cells := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(cols))
		for i := range cells {
			if cells[i].Valid {
				v := cells[i].String
				if maxTextLen > 0 && len([]rune(v)) > maxTextLen {
					row[i] = string([]rune(v)[:maxTextLen])
				} else {
					row[i] = v
				}
			} else {
				row[i] = ""
			}
		}
		vals = append(vals, row)
	}
	return cols, vals, rows.Err()
}
