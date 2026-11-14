package reporter

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// excelFile excelize 的轻封装，避免在每个单元格调用处重复错误处理。
type excelFile struct {
	f *excelize.File
}

func newExcel() *excelFile {
	ef := excelize.NewFile()
	_, _ = ef.NewSheet("概览")
	_, _ = ef.NewSheet("字段台账")
	_ = ef.DeleteSheet("Sheet1") // 移除默认空表
	return &excelFile{f: ef}
}

func (e *excelFile) setCell(sheet, coord, val string) {
	_ = e.f.SetCellValue(sheet, coord, val)
}

func (e *excelFile) save(path string) error {
	if err := e.f.SaveAs(path); err != nil {
		return fmt.Errorf("保存 Excel 报告失败：%w", err)
	}
	return nil
}

// colLetter 列索引（0 基）转 Excel 列字母：0→A，25→Z，26→AA。
func colLetter(i int) string {
	i++
	var s string
	for i > 0 {
		i--
		s = string(rune('A'+i%26)) + s
		i /= 26
	}
	return s
}
