// Command dbdlp 数据库敏感数据发现平台（只读扫描）入口。
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(resolveConfigDir()); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

// resolveConfigDir 在 cobra 解析前手动提取 --config（默认值 "."）。
func resolveConfigDir() string {
	dir := "."
	for i, a := range os.Args {
		if a == "--config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(a, "--config=") {
			return strings.TrimPrefix(a, "--config=")
		}
	}
	return dir
}

// run 装配应用并执行根命令；入口只做装配与退出码处理，不写业务。
func run(configDir string) error {
	app, err := NewApp(configDir)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.Root.Execute()
}
