// Package config 运行配置加载：dbdlp.toml 三级优先级 + 字段校验（docs/02 §8）。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dbdlp/internal/domain/port"
	"github.com/pelletier/go-toml/v2"
)

// Config 运行配置（与 docs/02 §8.2 全字段表对应）。
type Config struct {
	App     AppConfig     `toml:"app"`
	Log     LogConfig     `toml:"log"`
	Storage StorageConfig `toml:"storage"`
	Crypto  CryptoConfig  `toml:"crypto"`
	Scan    ScanConfig    `toml:"scan"`
	Engine  EngineConfig  `toml:"engine"`
	Report  ReportConfig  `toml:"report"`
	Web     WebConfig     `toml:"web"`

	ConfigDir string // 配置目录（密钥/规则基准路径）
}

type AppConfig struct {
	Name     string `toml:"name"`
	Env      string `toml:"env"`
	Timezone string `toml:"timezone"`
}
type LogConfig struct {
	Level         string `toml:"level"`
	Dir           string `toml:"dir"`
	Console       bool   `toml:"console"`
	RetentionDays int    `toml:"retention_days"`
	FilePrefix    string `toml:"file_prefix"`
}
type StorageConfig struct {
	Driver        string `toml:"driver"`
	SQLitePath    string `toml:"sqlite_path"`
	WAL           bool   `toml:"wal"`
	BusyTimeoutMs int    `toml:"busy_timeout_ms"`
	MaxOpenConns  int    `toml:"max_open_conns"`
	PostgresDSN   string `toml:"postgres_dsn"`
}
type CryptoConfig struct {
	DefaultAlgorithm string `toml:"default_algorithm"`
	KeyFile          string `toml:"key_file"`
	KeyFileMode      int    `toml:"key_file_mode"`
}
type ScanConfig struct {
	SampleRows      int    `toml:"sample_rows"`
	BatchSize       int    `toml:"batch_size"`
	Concurrency     int    `toml:"concurrency"`
	BatchIntervalMs int    `toml:"batch_interval_ms"`
	QueryTimeoutMs  int    `toml:"query_timeout_ms"`
	TaskTimeoutMin  int    `toml:"task_timeout_min"`
	MaxTextLen      int    `toml:"max_text_len"`
	LargeTableRows  int64  `toml:"large_table_rows"`
	SkipBinary      bool   `toml:"skip_binary"`
	LowPeakWindow   string `toml:"low_peak_window"`
	RetryMax        int    `toml:"retry_max"`
}
type EngineConfig struct {
	RulesFile        string  `toml:"rules_file"`
	DefaultThreshold float64 `toml:"default_threshold"`
	EnableNER        bool    `toml:"enable_ner"`
}
type ReportConfig struct {
	OutDir      string   `toml:"out_dir"`
	Formats     []string `toml:"formats"`
	MaskSamples bool     `toml:"mask_samples"`
}
type WebConfig struct {
	Enabled       bool   `toml:"enabled"`
	Listen        string `toml:"listen"`
	SessionTTLMin int    `toml:"session_ttl_min"`
	StaticDir     string `toml:"static_dir"`
}

func defaults() Config {
	return Config{
		App:     AppConfig{Name: "dbdlp", Env: "prod", Timezone: "Local"},
		Log:     LogConfig{Level: "INFO", Dir: "logs", Console: true, RetentionDays: 32, FilePrefix: "dbdlp"},
		Storage: StorageConfig{Driver: "sqlite", SQLitePath: "data/dbdlp.db", WAL: true, BusyTimeoutMs: 5000, MaxOpenConns: 1},
		Crypto:  CryptoConfig{DefaultAlgorithm: "aes256gcm", KeyFile: ".dbdlp/key", KeyFileMode: 384},
		Scan:    ScanConfig{SampleRows: 20, BatchSize: 20, Concurrency: 1, BatchIntervalMs: 50, QueryTimeoutMs: 30000, TaskTimeoutMin: 120, MaxTextLen: 2000, LargeTableRows: 10000000, SkipBinary: true, LowPeakWindow: "22:00-06:00", RetryMax: 3},
		Engine:  EngineConfig{RulesFile: "configs/rules.yaml", DefaultThreshold: 0.5, EnableNER: false},
		Report:  ReportConfig{OutDir: "reports", Formats: []string{"excel", "html", "json"}, MaskSamples: true},
		Web:     WebConfig{Enabled: true, Listen: "127.0.0.1:8080", SessionTTLMin: 480, StaticDir: "web/out"},
	}
}

// Load 三级优先级：CLI 覆盖 > toml 配置 > 内置默认。
// configDir 为 dbdlp.toml / .dbdlp/key / configs/ 的基准目录；为空时取当前目录。
func Load(configDir string) (*Config, error) {
	cfg := defaults()
	if configDir == "" {
		configDir = "."
	}
	if !filepath.IsAbs(configDir) {
		if abs, err := filepath.Abs(configDir); err == nil {
			configDir = abs
		}
	}
	cfg.ConfigDir = configDir

	tomlPath := filepath.Join(configDir, "dbdlp.toml")
	if data, err := os.ReadFile(tomlPath); err == nil {
		dec := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("配置文件存在未识别字段或格式错误，启动终止：%w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取 dbdlp.toml 失败：%w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate 字段约束校验（docs/02 §8.3）。
func (c *Config) validate() error {
	levels := map[string]bool{"TRACE": true, "DEBUG": true, "INFO": true, "WARN": true, "ERROR": true}
	if !levels[strings.ToUpper(c.Log.Level)] {
		return fmt.Errorf("日志级别非法：%s（仅允许 TRACE/DEBUG/INFO/WARN/ERROR）", c.Log.Level)
	}
	c.Log.Level = strings.ToUpper(c.Log.Level)
	if c.Log.RetentionDays < 32 {
		return fmt.Errorf("日志保留天数不允许小于 32，当前：%d", c.Log.RetentionDays)
	}
	if c.Scan.SampleRows < 1 || c.Scan.SampleRows > 1000 {
		return fmt.Errorf("采样行数必须为 1~1000，当前：%d", c.Scan.SampleRows)
	}
	if c.Scan.Concurrency < 1 || c.Scan.Concurrency > 4 {
		return fmt.Errorf("并发度必须为 1~4，当前：%d", c.Scan.Concurrency)
	}
	if c.Scan.BatchSize < 1 || c.Scan.BatchSize > 500 || c.Scan.BatchSize > c.Scan.SampleRows {
		return fmt.Errorf("批大小必须为 1~500 且 ≤ 采样行数，当前：%d", c.Scan.BatchSize)
	}
	if c.Scan.MaxTextLen < 1 || c.Scan.MaxTextLen > 65535 {
		return fmt.Errorf("最大文本长度必须为 1~65535，当前：%d", c.Scan.MaxTextLen)
	}
	if c.Crypto.DefaultAlgorithm != "aes256gcm" && c.Crypto.DefaultAlgorithm != "sm4gcm" {
		return fmt.Errorf("默认加密算法非法：%s（仅允许 aes256gcm/sm4gcm）", c.Crypto.DefaultAlgorithm)
	}
	if c.Storage.Driver != "sqlite" && c.Storage.Driver != "postgres" {
		return fmt.Errorf("存储驱动非法：%s", c.Storage.Driver)
	}
	if c.Storage.Driver == "sqlite" && c.Storage.MaxOpenConns != 1 {
		return fmt.Errorf("sqlite 存储 max_open_conns 必须为 1，当前：%d", c.Storage.MaxOpenConns)
	}
	if c.Web.Listen == "" {
		return fmt.Errorf("web 监听地址不能为空")
	}
	return nil
}

// DefaultAlgorithm 返回 port.Algorithm。
func (c *Config) DefaultAlgorithm() port.Algorithm {
	if c.Crypto.DefaultAlgorithm == "sm4gcm" {
		return port.AlgSM4GCM
	}
	return port.AlgAES256GCM
}

// RulesFilePath 规则文件绝对路径（不存在时返回空，引擎回退内置规则）。
func (c *Config) RulesFilePath() string {
	if c.Engine.RulesFile == "" {
		return ""
	}
	p := c.Engine.RulesFile
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.ConfigDir, p)
	}
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}
