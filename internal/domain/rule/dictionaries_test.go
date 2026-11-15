package rule

import "testing"

func TestMatchPerson(t *testing.T) {
	pos := []string{"张伟", "王芳", "欧阳建国", "上官云珠", "司马南", "李娜"}
	neg := []string{"北京市", "有限公司", "今天开会", "13812345678", "李", "计算机科学与技术"}
	for _, v := range pos {
		if !MatchPerson(v, personDict) {
			t.Errorf("期望命中人名：%s", v)
		}
	}
	for _, v := range neg {
		if MatchPerson(v, personDict) {
			t.Errorf("期望排除：%s", v)
		}
	}
}

func TestMatchCompany(t *testing.T) {
	pos := []string{"北京示例科技有限公司", "上海云图信息技术股份有限公司", "成都锦程软件开发合伙企业"}
	neg := []string{"这家公司", "科技公司", "银行", "上海市浦东新区世纪大道100号"}
	for _, v := range pos {
		if !MatchCompany(v, companyDict) {
			t.Errorf("期望命中公司：%s", v)
		}
	}
	for _, v := range neg {
		if MatchCompany(v, companyDict) {
			t.Errorf("期望排除：%s", v)
		}
	}
}

func TestMatchAddress(t *testing.T) {
	pos := []string{"北京市朝阳区建国路88号", "上海市徐汇区中山南二路1000号", "成都市武侯区天府大道北段966号3栋2单元501室"}
	neg := []string{"张三", "计算机", "今天下午三点开会讨论", "123456"}
	for _, v := range pos {
		if !MatchAddress(v, addressDict) {
			t.Errorf("期望命中地址：%s", v)
		}
	}
	for _, v := range neg {
		if MatchAddress(v, addressDict) {
			t.Errorf("期望排除：%s", v)
		}
	}
}
