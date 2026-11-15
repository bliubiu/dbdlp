// dictionaries.go 语义类（人名/公司名/地址）词典资源与整串结构判定。
// ADR-004：值级判定（候选值本身即判定对象，不做文本内切分），置信度封顶「中」并强制复核。
package rule

import "strings"

// personDict / companyDict / addressDict 内置语义词典（builtin 规则引用；
// YAML 自定义规则的 dictionary 段可在此基础上增补词条）。
var (
	personDict  = strings.Fields("王 李 张 刘 陈 杨 黄 赵 吴 周 徐 孙 马 朱 胡 郭 何 高 林 罗") // 人名判定使用内置姓氏表，此处为代表词条
	companyDict = strings.Split(companySuffixesRaw, " ")
	addressDict = strings.Split(addressWordsRaw, " ")
)

const companySuffixesRaw = "有限公司 有限责任公司 股份有限公司 集团公司 集团 合伙企业 工作室 研究院 研究所 事务所"
const addressWordsRaw = "省 自治区 市 区 县 镇 乡 街道 路 街 巷 号 号楼 栋 单元 室 层 座 大道 广场 大厦 园区 花园 小区 院"

// surnames 常见单姓（覆盖绝大多数中文姓氏）。
const surnames = "王李张刘陈杨黄赵吴周徐孙马朱胡郭何高林罗郑梁谢宋唐许韩冯邓曹彭曾肖田董袁潘于蒋蔡余杜叶程苏魏吕丁任沈姚卢姜崔钟谭陆汪范金石廖贾夏韦付方白邹孟熊秦邱江尹薛闫段雷侯龙史陶黎贺顾毛郝龚邵万钱严覃武戴莫孔向汤常温康施文牛樊葛邢安齐易乔伍庞颜倪庄聂章鲁岳翟殷詹申欧耿关兰焦俞左柳甘祝包宁尚符舒阮柯纪梅童凌毕单季裴霍涂成苗谷盛曲翁冉骆蓝路游辛靳管柴蒙鲍华喻祁蒲房滕屈饶解牟艾尤阳时穆农司卓古吉缪简车项连芦麦褚娄窦戚岑景党宫费卜冷晏席卫米柏宗瞿桂全佟应臧闵邬边卞姬师和仇栾隋商刁沙荣巫寇桑郎甄丛仲虞敖巩明佘池查麻苑迟邝官"

// compoundSurnames 复姓。
var compoundSurnames = strings.Fields("欧阳 上官 司马 诸葛 夏侯 皇甫 尉迟 公孙 长孙 慕容 司徒 司空 申屠 东方 独孤 南宫 万俟 闻人 西门 令狐 轩辕 呼延 鲜于 宇文 澹台 公冶 宗政 濮阳 淳于 单于 太叔 公羊 赫连")

// companySuffixes 公司名后缀（值级判定要求前缀 ≥4 汉字，防「这家公司」类误报）。
var companySuffixes = strings.Fields("有限公司 有限责任公司 股份有限公司 集团公司 集团 合伙企业 工作室 研究院 研究所 事务所")

// adminUnits 区划层级词（地址判定要求至少命中一个）。
var adminUnits = strings.Fields("省 市 区 县 镇 乡 旗 盟 州")

// addrStructWords 地址结构词（要求与区划词合计命中 ≥2 个不同词）。
var addrStructWords = strings.Fields("区 县 镇 乡 街道 路 街 巷 号 号楼 栋 单元 室 层 座 大道 广场 大厦 园区 花园 小区 院 州 省")

// allHan 检查整串是否全部为汉字（允许间隔号·，用于少数民族人名）。
func allHan(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if !(r >= 0x4E00 && r <= 0x9FFF) && r != '·' {
			return false
		}
	}
	return true
}

// personExcludes 人名排除特征词（机构/区划双字词，避免「司马」被单字「司」误伤）。
var personExcludes = []string{"公司", "有限", "集团", "研究院", "研究所", "大学", "医院", "银行", "中心"}

// MatchPerson 人名判定：全汉字 2~4 字，首字（或前两字）为姓氏，且不含机构/区划特征词。
func MatchPerson(v string, _ []string) bool {
	rs := []rune(v)
	n := len(rs)
	if n < 2 || n > 4 || !allHan(v) {
		return false
	}
	for _, w := range personExcludes {
		if strings.Contains(v, w) {
			return false
		}
	}
	first := string(rs[0])
	if n >= 3 && strings.Contains(compoundSurnamesStr, string(rs[:2])) {
		return true
	}
	return strings.Contains(surnames, first) && first != ""
}

var compoundSurnamesStr = strings.Join(compoundSurnames, "")

// MatchCompany 公司名判定：全汉字，以后缀词典结尾，且前缀 ≥4 汉字。
func MatchCompany(v string, suffixes []string) bool {
	if !allHan(v) {
		return false
	}
	for _, suf := range suffixes {
		sr := []rune(suf)
		if len(sr) == 0 {
			continue
		}
		if strings.HasSuffix(v, suf) && len([]rune(v))-len(sr) >= 4 {
			return true
		}
	}
	return false
}

// MatchAddress 地址判定：汉字/数字组成，含至少一个区划层级词，且结构词＋区划词合计 ≥2 个不同词。
func MatchAddress(v string, _ []string) bool {
	if v == "" || !allHanDigit(v) {
		return false
	}
	adminHit := false
	structHit := 0
	for _, w := range adminUnits {
		if strings.Contains(v, w) {
			adminHit = true
			break
		}
	}
	for _, w := range addrStructWords {
		if strings.Contains(v, w) {
			structHit++
		}
	}
	return adminHit && structHit >= 2
}

// allHanDigit 全部为汉字或数字（地址可含门牌号）。
func allHanDigit(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if !(r >= 0x4E00 && r <= 0x9FFF) && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// MatchDictionary 词典规则统一入口；按 DictKind 分派。
func MatchDictionary(kind, value string, words []string) bool {
	switch kind {
	case "person":
		return MatchPerson(value, words)
	case "company":
		return MatchCompany(value, words)
	case "address":
		return MatchAddress(value, words)
	}
	return false
}
