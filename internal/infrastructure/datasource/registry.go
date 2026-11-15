package datasource

import (
	"fmt"

	"dbdlp/internal/domain/port"
)

// NewDriver 按方言返回驱动实现；未知方言返回错误。
func NewDriver(d port.Dialect) (port.Driver, error) {
	switch d {
	case port.DialectOracle:
		return &oracleDriver{}, nil
	case port.DialectDameng:
		return &damengDriver{}, nil
	case port.DialectMySQL:
		return &mysqlDriver{}, nil
	case port.DialectPostgres:
		return &postgresDriver{}, nil
	case port.DialectSQLite:
		return &sqliteDriver{}, nil
	default:
		return nil, fmt.Errorf("暂不支持的方言：%s", d)
	}
}

// ExcludedSchemasOracle 已确认的系统模式排除清单（docs/02 §5.3.1）。
var ExcludedSchemasOracle = []string{"SYS", "SYSTEM", "SYSAUX", "SYSDBA", "SYSAUDITOR", "SYSSSO"}

// ExcludedSchemasDameng 达梦核心系统模式（e2e-dm8 实测）。
var ExcludedSchemasDameng = []string{"SYS", "SYSDBA", "SYSAUDITOR", "SYSSSO"}
