package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "INFO" {
		t.Errorf("默认日志级别应为 INFO，got %s", cfg.Log.Level)
	}
	if cfg.Scan.SampleRows != 20 {
		t.Errorf("默认采样行数应为 20，got %d", cfg.Scan.SampleRows)
	}
	if cfg.Storage.Driver != "sqlite" || cfg.Storage.MaxOpenConns != 1 {
		t.Errorf("默认存储应为 sqlite 单写者")
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"bad_level":   "[log]\nlevel = \"VERBOSE\"\n",
		"small_ret":   "[log]\nretention_days = 10\n",
		"sample_zero": "[scan]\nsample_rows = 0\n",
		"concurrency": "[scan]\nconcurrency = 9\n",
		"bad_alg":     "[crypto]\ndefault_algorithm = \"des\"\n",
	}
	for name, content := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "dbdlp.toml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Errorf("%s 应通过校验报错，但没报", name)
		}
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	content := "[log]\nlevel = \"INFO\"\nbogus_field = 1\n"
	if err := os.WriteFile(filepath.Join(dir, "dbdlp.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Errorf("未识别字段应导致启动终止")
	}
}

func TestTomlOverrideAndTier(t *testing.T) {
	dir := t.TempDir()
	content := "[scan]\nsample_rows = 50\n[log]\nlevel = \"debug\"\n"
	if err := os.WriteFile(filepath.Join(dir, "dbdlp.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scan.SampleRows != 50 {
		t.Errorf("toml 覆盖应生效，got %d", cfg.Scan.SampleRows)
	}
	if cfg.Log.Level != "DEBUG" {
		t.Errorf("toml 覆盖应生效，got %s", cfg.Log.Level)
	}
	// 未配置项仍取默认
	if cfg.Storage.SQLitePath != "data/dbdlp.db" {
		t.Errorf("未配置项应取默认")
	}
}
