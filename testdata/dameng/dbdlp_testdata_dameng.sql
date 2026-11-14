-- ============================================================================
-- dbdlp 敏感数据识别功能测试集（达梦版）
-- ----------------------------------------------------------------------------
-- 用途     : 在达梦测试库中构建敏感数据测试表，用于验证 dbdlp 扫描识别能力
-- 适用     : 达梦 DM8 及以上（本文件编码为 GB18030，disql 读文件固定按 GB18030 解码后转服务端字符集）
-- 执行方式 : disql 用户名/密码@主机:5236 `dbdlp_testdata_dameng.sql`
--            或 disql 连接后执行: START dbdlp_testdata_dameng.sql
-- 期望结果 : 各表结构、数据分布与逐列期望识别结果见 testdata/README.md
-- 编码说明 : disql START 装载须用 GB18030 编码文件；若源文件为 UTF-8 请先执行
--            iconv -f UTF-8 -t GB18030 输入.sql > 输出.sql
-- 数据说明 : 全部为构造数据，不含真实敏感信息；正样本校验位均按
--            dbdlp 校验器算法（身份证加权模11、银行卡Luhn、信用代码GB32100）生成
-- 幂等性   : 脚本可重复执行，开头自动清理同名旧表
-- ============================================================================

-- ============================================================================
-- 0. 清理旧表（首次执行时表不存在，匿名块静默跳过）
-- ============================================================================
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_CUSTOMER CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_PERSON CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_ENTERPRISE CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_NETWORK CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_VEHICLE CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_TEXT_LOB CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_NEGATIVE CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/
BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE T_DBDLP_TEST_BIGTABLE CASCADE CONSTRAINTS';
EXCEPTION WHEN OTHERS THEN NULL;
END;
/

-- ============================================================================
-- 1. 客户表 T_DBDLP_TEST_CUSTOMER（22 行）
--    覆盖: 人名(语义)/身份证/手机号/邮箱/银行卡/地址(语义) + 普通文本对照列
--    负样本行固定为 5/10/15/20，正样本行 18 行，单次 20 行抽样窗口全覆盖
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_CUSTOMER (
  CUST_ID      NUMBER        PRIMARY KEY,
  CUST_NAME    VARCHAR2(64),
  ID_CARD_NO   VARCHAR2(32),
  MOBILE_NO    VARCHAR2(32),
  EMAIL        VARCHAR2(128),
  BANK_CARD_NO VARCHAR2(32),
  REG_ADDRESS  VARCHAR2(256),
  MEMO         VARCHAR2(256)
);

COMMENT ON TABLE  T_DBDLP_TEST_CUSTOMER IS 'dbdlp测试-客户信息表';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.CUST_NAME    IS '客户姓名';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.ID_CARD_NO   IS '身份证号码';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.MOBILE_NO    IS '手机号';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.EMAIL        IS '电子邮箱';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.BANK_CARD_NO IS '银行卡号';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.REG_ADDRESS  IS '注册地址';
COMMENT ON COLUMN T_DBDLP_TEST_CUSTOMER.MEMO         IS '备注-非敏感对照列';

-- 行1~4：正样本
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (1, '张伟',   '110101199003071006', '13812345678', 'zhangsan@example.com',       '6222020000000007',    '北京市朝阳区建国路88号院3号楼2单元501室', '客户由华东区渠道推荐接入');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (2, '王芳',   '310104198512192377', '13800001234', 'lisi@163.com',                '6228480000079191',    '上海市徐汇区中山南二路1000号',           '2026年度续约客户，等级A级');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (3, '李娜',   '440305199711083748', '15912345678', 'wangwu@qq.com',               '6225760000158381',    '深圳市南山区科技园南区高新南一道12号',   '已开通短信通知服务');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (4, '欧阳建国','330106200103225111', '18600002222', 'zhaoliu@126.com',             '6217000000237574',    '杭州市西湖区文三路199号创业大厦8层',     '发票抬头已变更为公司主体');
-- 行5：负样本行（占位身份证/全同号手机/无@邮箱/订单号串）
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (5, '刘洋',   '110101199001011234', '11111111111', 'zhangsan#example.com',        '202611140000123456',  '成都市武侯区天府大道北段966号',           '测试占位数据请勿联系');
-- 行6~9：正样本
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (6, '陈静',   '510107198706156485', '13900001111', 'admin@dbdlp-test.org',        '6222600000316766',    '南京市玄武区中山东路300号',               '会员积分将于月底过期');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (7, '赵磊',   '320102199509017854', '17712348888', 'service@company.cn',          '6226660000395950',    '武汉市洪山区珞瑜路1037号',               '风险评测已完成');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (8, '孙丽',   '12010119820130922X', '13587654321', 'zhang.san.tag@gmail.com',     '6229210000475142',    '西安市碑林区南大街30号',                 '预留联系时段为工作日');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (9, '周杰',   '420106199912041597', '18811112222', 'li_si@yahoo.com.cn',          '6205220000554331',    '青岛市市南区香港中路76号',               '绑定代扣协议待生效');
-- 行10：负样本行（校验位错/非法号段/双@/银行卡末位错）
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (10, '吴敏',   '110101199003071005', '12345678901', 'a@b@c.com',                   '6222020000000008',    '厦门市思明区软件园二期观日路12号',       '历史工单已关闭');
-- 行11~14：正样本
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (11, '郑强',   '610103197407212962', '13699887766', 'wang-wu@outlook.com',         '6217850000000633524', '重庆市渝中区民生路318号',                 '升级为白名单用户');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (12, '冯雪',   '370202198810184336', '15012345678', 'u1@sub.example.org',          '6221500000000712712', '长沙市岳麓区麓山路155号',                 '参与新功能内测邀请');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (13, '蒋涛',   '350203199304055707', '18912340000', 'test@shili-tech.com',         '6221880000000791906', '合肥市蜀山区长江西路299号',               '账号迁移至新系统');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (14, '韩梅',   '500103200201167077', '13100000001', 'hr@shili.com.cn',             '6217730000000871096', '兰州市城关区庆阳路486号',                 '实名认证材料已归档');
-- 行15：负样本行（2月30日身份证/12位手机/无@邮箱/20位超长串）
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (15, '杨帆',   '110101199002307458', '138123456781','userexample.com',             '62220200000000078901','昆明市五华区东风西路167号',               '异常格式数据用于降误报验证');
-- 行16~19：正样本
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (16, '朱琳',   '430104198608238445', '17600112233', 'dev@internal.shili.cn',       '6217580000104729001', '南宁市青秀区民族大道137号',               '投诉工单转客服二组');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (17, '高翔',   '340104199606309812', '18012345678', 'sales@example2026.com',       '6221680000105776291', '哈尔滨市南岗区中山路248号',               '账单地址与注册地址一致');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (18, '林悦',   '210102197911112185', '13755556666', 'a.b@c.d.co',                  '6212260000106823586', '郑州市金水区花园路82号',                   '一次性告知材料清单已发送');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (19, '罗成',   '620102199108093558', '15288889999', 'ops@example.net',             '6229080107870879',    '贵阳市云岩区中华北路263号',               '专属客户经理已对接');
-- 行20：负样本行（15位旧号/过短手机/双@变体/含字母卡号）
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (20, '黄敏',   '110101900307100',    '1381234',     'user@@example.com',           '62220200000000O7',    '南昌市东湖区八一大道321号',               '15位旧号与异常格式验证');
-- 行21~22：超出默认抽样窗口行（验证主键分页与抽样截断），均为正样本
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (21, '徐静',   '530102198404274920', '19512345678', 'fin@shili-tech.com.cn',       '6229090108918163',    '沈阳市和平区北三经街47号',               '超出默认抽样窗口的补充行');
INSERT INTO T_DBDLP_TEST_CUSTOMER VALUES (22, '何军',   '450103200308146298', '19900001111', 'bd@shili-tech.com',           '6212830109965450',    '乌鲁木齐市天山区人民路158号',             '超出默认抽样窗口的补充行');

-- ============================================================================
-- 2. 人员表 T_DBDLP_TEST_PERSON（22 行）
--    覆盖: 人名/固话/邮编/地址(均为语义类与中低置信类)
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_PERSON (
  PERSON_ID    NUMBER        PRIMARY KEY,
  REAL_NAME    VARCHAR2(64),
  FIXED_PHONE  VARCHAR2(32),
  POST_CODE    VARCHAR2(16),
  HOME_ADDRESS VARCHAR2(256),
  REMARK       VARCHAR2(256)
);

COMMENT ON TABLE  T_DBDLP_TEST_PERSON IS 'dbdlp测试-人员信息表';
COMMENT ON COLUMN T_DBDLP_TEST_PERSON.REAL_NAME    IS '真实姓名';
COMMENT ON COLUMN T_DBDLP_TEST_PERSON.FIXED_PHONE  IS '固定电话';
COMMENT ON COLUMN T_DBDLP_TEST_PERSON.POST_CODE    IS '邮政编码';
COMMENT ON COLUMN T_DBDLP_TEST_PERSON.HOME_ADDRESS IS '家庭住址';
COMMENT ON COLUMN T_DBDLP_TEST_PERSON.REMARK       IS '备注-非敏感对照列';

INSERT INTO T_DBDLP_TEST_PERSON VALUES (1,  '张伟',     '010-88886666', '100000', '北京市东城区长安街1号',           '在职员工');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (2,  '王芳',     '021-65554433', '100089', '北京市海淀区中关村大街27号',       '部门经理');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (3,  '李娜',     '010-62771234', '200001', '上海市黄浦区南京东路88号',         '外包驻场');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (4,  '刘洋',     '021-33080600', '300041', '天津市和平区南京路219号',          '实习生');
-- 负样本行5：人名=地名 / 固话无0开头 / 邮编7位
INSERT INTO T_DBDLP_TEST_PERSON VALUES (5,  '北京市朝阳区', '12345678',  '1000000','石家庄市桥西区中山西路666号',      '负样本行');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (6,  '陈静',     '0755-83456000', '310003','上海市浦东新区世纪大道100号',      '离职交接中');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (7,  '赵磊',     '027-65608000',  '330002','广州市天河区体育西路103号',        '休产假');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (8,  '孙丽',     '0571-87654321', '350003','深圳市福田区深南大道6008号',       '常驻深圳');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (9,  '宋佳',     '020-83331234', '710003', '苏州市姑苏区人民路750号',          '调岗申请已批准');
-- 负样本行10：人名=商品名 / 固话过短 / 邮编含字母
INSERT INTO T_DBDLP_TEST_PERSON VALUES (10, '华为手机', '0101234',    '10000A', '杭州市滨江区江南大道588号',        '负样本行');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (11, '周杰',     '025-83123456', '361001','成都市高新区天府软件园C座',        '项目负责人');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (12, '吴敏',     '029-87654321', '410005','南京市鼓楼区中山北路8号',          '出差至西安');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (13, '郑强',     '028-85553322', '430014','武汉市江汉区解放大道688号',        '本周居家办公');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (14, '谢峰',     '0731-84421234', '810001', '济南市历下区经十路63号',           '年度体检通知已发');
-- 负样本行15：人名=泛指词 / 固话带国家码 / 邮编5位
INSERT INTO T_DBDLP_TEST_PERSON VALUES (15, '有限公司', '86-010-88886666', '10000','长沙市芙蓉区五一大道335号',      '负样本行');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (16, '冯雪',     '022-27305678', '450003','青岛市市南区山东路29号',           '报销已到账');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (17, '蒋涛',     '023-63871234', '500011','重庆市江北区江北城西大街25号',     '会议请假一次');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (18, '韩梅',     '0411-84631234', '510030','厦门市湖里区吕岭路1263号',        '新入职员工');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (19, '邓丽',     '0512-65231234', '830002', '昆明市盘龙区北京路289号',          '工会活动报名中');
-- 负样本行20：人名=通用词 / 固话带分机 / 邮编含连字符
INSERT INTO T_DBDLP_TEST_PERSON VALUES (20, '小程序',   '010-88886666-8001', '100-00','西安市雁塔区高新路25号',       '负样本行');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (21, '欧阳建国', '024-23123456', '518000','沈阳市沈河区青年大街109号',        '复姓名字样本');
INSERT INTO T_DBDLP_TEST_PERSON VALUES (22, '上官云珠', '0532-85721234', '610015','广州市越秀区环市东路329号',        '复姓名字样本');

-- ============================================================================
-- 3. 企业表 T_DBDLP_TEST_ENTERPRISE（22 行）
--    覆盖: 公司名(语义)/统一社会信用代码/域名/邮箱
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_ENTERPRISE (
  ENT_ID            NUMBER        PRIMARY KEY,
  COMPANY_NAME      VARCHAR2(128),
  SOCIAL_CREDIT_CODE VARCHAR2(32),
  DOMAIN_NAME       VARCHAR2(128),
  CONTACT_EMAIL     VARCHAR2(128),
  BUSINESS_SCOPE    VARCHAR2(512)
);

COMMENT ON TABLE  T_DBDLP_TEST_ENTERPRISE IS 'dbdlp测试-企业信息表';
COMMENT ON COLUMN T_DBDLP_TEST_ENTERPRISE.COMPANY_NAME       IS '企业名称';
COMMENT ON COLUMN T_DBDLP_TEST_ENTERPRISE.SOCIAL_CREDIT_CODE IS '统一社会信用代码';
COMMENT ON COLUMN T_DBDLP_TEST_ENTERPRISE.DOMAIN_NAME        IS '企业域名';
COMMENT ON COLUMN T_DBDLP_TEST_ENTERPRISE.CONTACT_EMAIL      IS '联系邮箱';
COMMENT ON COLUMN T_DBDLP_TEST_ENTERPRISE.BUSINESS_SCOPE     IS '经营范围-非敏感对照列';

INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (1,  '北京示例科技有限公司',             '91110108000000MA3X', 'www.example.com',                'contact@example.com',         '技术开发、技术咨询、技术服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (2,  '上海云图信息技术有限公司',         '91310115002749MA3X', 'www.shili-tech.com',             'info@shili-tech.com',         '软件开发；系统集成；数据处理');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (3,  '深圳市前海数联科技股份有限公司',   '91440300005498MA3X', 'api.shili-tech.com.cn',          'support@shili-tech.com.cn',   '云计算装备技术服务；互联网数据服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (4,  '杭州西子智能装备有限公司',         '91330106008247MA3W', 'mail.company.org',               'mail@company.org',            '智能装备制造；工业机器人销售');
-- 负样本行5：公司名泛指句 / 信用代码末位错 / 域名无顶级域
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (5,  '这家公司很棒',                     '91110108000000MA39', 'example',                        'root@example.com',             '负样本行');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (6,  '成都锦程软件开发有限公司',         '91510107010996MA37', 'static.cdn.example.net',         'cdn@example.net',              '计算机软件开发；网络技术推广');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (7,  '南京金陵数据服务有限公司',         '91320102013745MA39', 'blog.developer.shili.io',        'blog@shili.io',                '数据处理与存储服务；大数据分析');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (8,  '天津海河物流有限公司',             '91420106016494MA3E', 'www.internal.corp.cn',           'ops@internal.corp.cn',         '道路货物运输；仓储服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (9,  '兰州黄河环保科技有限公司',         '91650102151195MA3A', 'www.green-env-tech.com',        'service@green-env-tech.com',   '环保设备研发；水污染治理');
-- 负样本行10：公司名泛指 / 信用代码17位 / 邮箱本地误当域名
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (10, '个体工商户',                       '91110108MA3X00000',  'user@example.com',               'a@b@c.org',                    '负样本行');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (11, '武汉长江网络科技有限公司',         '91610103019243MA31', 'ns1.example-dns.com',            'admin@example-dns.com',        '网络技术服务；信息安全软件开发');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (12, '西安古都文化传媒有限公司',         '91370202021992MA3K', 'cdn.static-assets.example.org',  'assets@example.org',           '文化艺术交流活动策划；影视制作');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (13, '青岛海洋生物科技有限公司',         '91350203024741MA3X', 'api-v2.gateway.shili.com',       'gateway@shili.com',            '海洋生物制品研发；检验检测服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (14, '昆明春城生物制药有限公司',         '91460100153944MA3D', 'api.biotech-spring.cn',         'research@biotech-spring.cn',   '生物药品制造；药品互联网信息服务');
-- 负样本行15：公司名泛指句 / 信用代码19位 / 域名标签以连字符开头
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (15, '互联网公司数量很多',               '91110108000000MA3X0','-example.com',                   'b@c.org',                      '负样本行');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (16, '厦门鹭岛电子商务有限公司',         '91110109137450MA3H', 'www.example2026.net',            'shop@example2026.net',         '互联网销售；供应链管理服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (17, '重庆山城智能科技有限公司',         '91210113140199MA33', 'smtp.mail.example.com',          'postmaster@example.com',       '人工智能应用软件开发；物联网设备销售');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (18, '长沙湘江信息科技有限公司',         '91410102142948MA3G', 'git.internal.shili.dev',         'scm@shili.dev',                '信息技术咨询服务；软件外包服务');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (19, '南宁绿城农业开发有限公司',         '91340104156693MA38', 'shop.green-city-agri.com',      'trade@green-city-agri.com',    '农产品初加工；农业技术研发');
-- 负样本行20：公司名通用句 / 信用代码含非法字符I / 域名双点
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (20, '公司年会通知',                     '91110108000000IA3X', 'example..com',                   'x@y.z',                        '负样本行');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (21, '合肥庐州教育科技有限公司',         '91370100145697MA31', 'docs.wiki.shili.cn',             'wiki@shili.cn',                '教育软件研发；在线教育培训');
INSERT INTO T_DBDLP_TEST_ENTERPRISE VALUES (22, '沈阳北国机械制造有限公司',         '91530112148446MA38', 's3.storage.example.cloud',       'storage@example.cloud',        '机械设备制造；模具加工');

-- ============================================================================
-- 4. 网络资产表 T_DBDLP_TEST_NETWORK（22 行）
--    覆盖: IPv4(含127.0.0.1保留场景)/IPv6/MAC/域名
--    说明: IPv6/MAC 为规则预留数据，当前内置规则未含时不应误判
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_NETWORK (
  NET_ID      NUMBER        PRIMARY KEY,
  IPV4_ADDR   VARCHAR2(64),
  IPV6_ADDR   VARCHAR2(128),
  MAC_ADDR    VARCHAR2(64),
  HOST_DOMAIN VARCHAR2(128),
  SERVER_DESC VARCHAR2(512)
);

COMMENT ON TABLE  T_DBDLP_TEST_NETWORK IS 'dbdlp测试-网络资产表';
COMMENT ON COLUMN T_DBDLP_TEST_NETWORK.IPV4_ADDR   IS 'IPv4地址';
COMMENT ON COLUMN T_DBDLP_TEST_NETWORK.IPV6_ADDR   IS 'IPv6地址';
COMMENT ON COLUMN T_DBDLP_TEST_NETWORK.MAC_ADDR    IS 'MAC地址';
COMMENT ON COLUMN T_DBDLP_TEST_NETWORK.HOST_DOMAIN IS '主机域名';
COMMENT ON COLUMN T_DBDLP_TEST_NETWORK.SERVER_DESC IS '服务器说明-非敏感对照列';

INSERT INTO T_DBDLP_TEST_NETWORK VALUES (1,  '192.168.10.3',   '2001:db8::1',                              'AA:BB:CC:DD:EE:FF', 'www.example.com',        '生产环境应用服务器');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (2,  '10.0.0.1',       '2001:0db8:0000:0000:0000:0000:0000:0001',  'aa:bb:cc:dd:ee:ff', 'www.shili-tech.com',     '核心数据库服务器');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (3,  '172.16.8.100',   '::ffff:192.168.1.1',                       'AA-BB-CC-DD-EE-FF', 'api.shili-tech.com.cn',  '内网测试服务器');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (4,  '8.8.8.8',        'fe80::1',                                  '00-1A-2B-3C-4D-5E', 'mail.company.org',       '外部DNS服务器');
-- 负样本行5：IPv4超255 / IPv6双冒号 / MAC仅5组
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (5,  '256.1.1.1',      '2001::db8::a',                             'AA:BB:CC:DD:EE',    'example',                '负样本行');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (6,  '114.114.114.114','2001:db8:0:0:0:0:2:1',                     '0a:1b:2c:3d:4e:5f', 'static.cdn.example.net', '公共DNS备用节点');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (7,  '192.168.1.254',  '::1',                                      'FF:FF:FF:FF:FF:FF', 'blog.developer.shili.io','网关广播地址测试');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (8,  '172.20.10.5',    '2001:db8:85a3::8a2e:370:7334',             '08:00:27:aa:bb:cc', 'www.internal.corp.cn',   '回环地址-脱敏保留场景');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (9,  '10.1.2.3',      'fd00:abcd::1',                             'dc:a6:32:aa:bb:cc', 'auth.sso.shili.com',     '容器平台管理节点');
-- 负样本行10：IPv4段数不足 / IPv6共9组 / MAC含字母G
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (10, '10.0.0',         '2001:db8:0:0:0:0:0:1:2',                   'AA:BB:CG:DD:EE:FF', 'user@example.com',       '负样本行');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (11, '10.10.10.10',    'fd00::1234',                               'D4:BE:D9:12:34:56', 'ns1.example-dns.com',    '虚拟化宿主机');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (12, '192.168.100.20', '2001:db8::8a2e:370:7334',                  '3c:97:0e:aa:bb:cc', 'cdn.static-assets.example.org', '文件同步节点');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (13, '172.16.254.1',   'fe80::a:bbcc:ddee:ff00',                   '00:0c:29:12:34:56', 'api-v2.gateway.shili.com','IPv6过渡网段');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (14, '192.168.0.100', '2001:db8:ffff::5',                         'e4:5f:01:ab:cd:ef', 'monitor.ops.example.cn', '日志汇聚服务器');
-- 负样本行15：IPv4段数过多 / IPv6非法十六进制 / MAC含空段
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (15, '192.168.1.1.1',  'gggg::1',                                  'AA::BB:CC:DD:EE:FF','bbs.example.com',        '负样本行');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (16, '8.8.4.4',        'fc00::5678',                               'A4:5E:60:BB:CC:DD', 'git.internal.shili.dev', '二级DNS节点');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (17, '223.5.5.5',      '2001:db8:a::b:c',                          '00:16:3e:aa:bb:cc', 'docs.wiki.shili.cn',     '公共DNS主节点');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (18, '10.0.5.100',     'fd12:3456:789a::1',                        '52:54:00:12:34:56', 's3.storage.example.cloud','K8S节点测试网段');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (19, '10.20.30.40',   'fe80::1234:5678:9abc:def0',                '00:1b:44:11:3a:b7', 'backup.svc.example.net', '备份存储服务器');
-- 负样本行20：IPv4非数字 / IPv6全压缩空地址 / MAC双冒号空段
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (20, 'abc.def.ghi.jkl','::',                                       '::BB:CC:DD:EE:FF',  'example..com',           '负样本行');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (21, '192.168.50.50',  '::ffff:10.0.0.1',                          '00:50:56:c0:00:08', 'auth.sso.shili.com',     '回环映射测试地址');
INSERT INTO T_DBDLP_TEST_NETWORK VALUES (22, '223.6.6.6',      '2001:db8:1:2:3:4:5:6',                     'b8:27:eb:12:34:56', 'monitor.ops.example.cn', '监控告警服务器');

-- ============================================================================
-- 5. 出行表 T_DBDLP_TEST_VEHICLE（22 行）
--    覆盖: 车牌(含新能源)/护照/车主姓名
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_VEHICLE (
  TRAVEL_ID   NUMBER        PRIMARY KEY,
  PLATE_NO    VARCHAR2(32),
  PASSPORT_NO VARCHAR2(32),
  OWNER_NAME  VARCHAR2(64),
  TRIP_NOTE   VARCHAR2(512)
);

COMMENT ON TABLE  T_DBDLP_TEST_VEHICLE IS 'dbdlp测试-出行信息表';
COMMENT ON COLUMN T_DBDLP_TEST_VEHICLE.PLATE_NO    IS '车牌号码';
COMMENT ON COLUMN T_DBDLP_TEST_VEHICLE.PASSPORT_NO IS '护照号码';
COMMENT ON COLUMN T_DBDLP_TEST_VEHICLE.OWNER_NAME  IS '车主姓名';
COMMENT ON COLUMN T_DBDLP_TEST_VEHICLE.TRIP_NOTE   IS '行程备注-非敏感对照列';

INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (1,  '京A12345', 'E12345678', '张伟',     '工作日通勤出行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (2,  '沪B67890', 'G23456789', '王芳',     '出差上海虹桥机场');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (3,  '粤B88888', 'E34567890', '李娜',     '客户现场支持');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (4,  '苏E05D88', 'G45678901', '刘洋',     '周末郊游');
-- 负样本行5：车牌省份非法 / 护照数字仅7位
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (5,  '森A12345', 'E1234567',  '陈静',     '负样本行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (6,  '浙A13579', 'E56789012', '赵磊',     '加油卡已充值');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (7,  '鲁B24680', 'P12345678', '孙丽',     '年检预约成功');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (8,  '豫A98765', 'S12345678', '周杰',     'ETC账单已出');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (9,  '鄂A66666', 'G01234567', '徐静',     '试驾预约登记');
-- 负样本行10：车牌第二位非字母 / 护照数字9位
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (10, '京123456', 'E123456789','吴敏',     '负样本行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (11, '川A1B234', 'D12345678', '郑强',     '新能源车牌样本');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (12, '京AD12345', 'E67890123','冯雪',     '新能源大型车牌样本');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (13, '粤BF67890', 'G78901234', '蒋涛',     '新能源小型车牌样本');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (14, '黑A77777', 'P23456789', '何军',     '冬季轮胎更换提醒');
-- 负样本行15：车牌缺省份 / 护照纯数字
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (15, 'A12345',   '123456789', '韩梅',     '负样本行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (16, '陕A11111', 'E89012345', '杨帆',     '限行尾号轮换');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (17, '湘C22222', 'G89012345', '朱琳',     '停车场月卡续费');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (18, '闽D33333', 'E90123456', '高翔',     '异地用车申请');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (19, '新A88888', 'S23456789', '黄敏',     '长途出行报备');
-- 负样本行20：车牌总长不足 / 护照尾部为字母
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (20, '京A1234',  'E1234567A', '林悦',     '负样本行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (21, '皖A44444', 'G90123456', '罗成',     '跨省长途出行');
INSERT INTO T_DBDLP_TEST_VEHICLE VALUES (22, '赣B55555', 'E01234567', '黄敏',     '代驾服务记录');

-- ============================================================================
-- 6. 文本边界表 T_DBDLP_TEST_TEXT_LOB（6 行）
--    覆盖: JSON内嵌(BD-01)/CLOB超长截断(BD-02)/中文上下文(BD-03)/空值族(BD-05)/已掩码值(NG-04)
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_TEXT_LOB (
  TEXT_ID      NUMBER         PRIMARY KEY,
  JSON_PAYLOAD CLOB,
  LONG_TEXT    CLOB,
  PLAIN_TEXT   VARCHAR2(2000)
);

COMMENT ON TABLE  T_DBDLP_TEST_TEXT_LOB IS 'dbdlp测试-文本边界表';
COMMENT ON COLUMN T_DBDLP_TEST_TEXT_LOB.JSON_PAYLOAD IS 'JSON载荷-内嵌敏感数据';
COMMENT ON COLUMN T_DBDLP_TEST_TEXT_LOB.LONG_TEXT    IS '超长文本-截断场景';
COMMENT ON COLUMN T_DBDLP_TEST_TEXT_LOB.PLAIN_TEXT   IS '普通文本-非敏感对照';

INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (1,
  '{"custName":"张三","idCard":"110101199003071006","mobile":"13812345678","bankCard":"6222020000000007","email":"zhangsan@example.com"}',
  NULL,
  NULL);

-- 行2：超长文本由匿名块生成（约4000字节中文，中部嵌入手机号与身份证），此处先占位
INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (2,
  NULL,
  EMPTY_CLOB(),
  NULL);

INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (3,
  NULL,
  '联系人张三，手机13812345678，身份证110101199003071006，住址北京市朝阳区建国路88号，银行卡6222020000000007，请尽快联系办理。',
  '这是一段普通业务说明文本，用于验证非敏感列不命中。');

INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (4,
  '{"custName":"李四","idCard":"1101**********1234","mobile":"138****1234","ip":"*.*.10.3","bankCard":"6222**********1234"}',
  NULL,
  '已掩码数据回存场景：验证掩码值不二次命中、不还原。');

-- 行5：空值族（NULL/空串）
INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (5,
  NULL,
  '',
  NULL);

INSERT INTO T_DBDLP_TEST_TEXT_LOB VALUES (6,
  NULL,
  NULL,
  '本季度销售额同比增长百分之十五，请各部门注意排查库存与应收账款，会议定于周三下午两点在第三会议室召开。');

-- 行2 超长文本填充：重复中文段落至约4000字节后追加含敏感数据的上下文句
DECLARE
  v_txt VARCHAR2(32000);
BEGIN
  v_txt := RPAD('这是一段用于撑长的中文业务说明文本，模拟真实系统中的日志或备注内容。', 3000, '内容含多种信息但不构成完整敏感数据。');
  v_txt := v_txt || v_txt;
  v_txt := v_txt || '联系人张三，手机13812345678，身份证110101199003071006，请尽快联系。';
  UPDATE T_DBDLP_TEST_TEXT_LOB SET LONG_TEXT = v_txt WHERE TEXT_ID = 2;
  COMMIT;
END;
/

-- ============================================================================
-- 7. 通用负样本池 T_DBDLP_TEST_NEGATIVE（20 行，NG-01~NG-06 轮转）
--    期望: 逐条对照 README 验收口径——被判高风险数必须为 0
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_NEGATIVE (
  NG_ID       NUMBER        PRIMARY KEY,
  SAMPLE_TYPE VARCHAR2(16),
  SAMPLE_VALUE VARCHAR2(512)
);

COMMENT ON TABLE  T_DBDLP_TEST_NEGATIVE IS 'dbdlp测试-通用负样本池';
COMMENT ON COLUMN T_DBDLP_TEST_NEGATIVE.SAMPLE_TYPE  IS '负样本类别编号';
COMMENT ON COLUMN T_DBDLP_TEST_NEGATIVE.SAMPLE_VALUE IS '负样本值';

INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (1,  'NG-01', '202611140000123456');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (2,  'NG-01', '2026010112345678');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (3,  'NG-01', 'SO202600001234');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (4,  'NG-01', '10000012345678901');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (5,  'NG-01', '98765432109876543210');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (6,  'NG-02', '90123456789');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (7,  'NG-02', '1357924680135792');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (8,  'NG-02', '246813579024681357');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (9,  'NG-02', '1234567890123456789');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (10, 'NG-03', '110101199001011234');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (11, 'NG-03', '0000000000000000');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (12, 'NG-03', '111111111111111111');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (13, 'NG-03', '99999999999999999');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (14, 'NG-04', '138****1234');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (15, 'NG-04', '1101**********1234');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (16, 'NG-04', '6222**********1234');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (17, 'NG-04', '*.*.10.3');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (18, 'NG-05', '本季度销售额同比增长百分之十五，请各部门注意排查。');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (19, 'NG-05', '会议定于周三下午两点在第三会议室召开。');
INSERT INTO T_DBDLP_TEST_NEGATIVE VALUES (20, 'NG-06', '550e8400-e29b-41d4-a716-446655440000');

-- ============================================================================
-- 8. 分页大表 T_DBDLP_TEST_BIGTABLE（1000 行，SQL 表达式生成）
--    覆盖: 主键分页/断点续扫/流式读取(T-IT-03)；手机号/邮箱/IPv4 命中验证
-- ============================================================================
CREATE TABLE T_DBDLP_TEST_BIGTABLE (
  ID         NUMBER        PRIMARY KEY,
  MOBILE_NO  VARCHAR2(32),
  EMAIL      VARCHAR2(128),
  IPV4_ADDR  VARCHAR2(64),
  ORDER_NO   VARCHAR2(32),
  CUST_LEVEL NUMBER
);

COMMENT ON TABLE  T_DBDLP_TEST_BIGTABLE IS 'dbdlp测试-分页大表';
COMMENT ON COLUMN T_DBDLP_TEST_BIGTABLE.MOBILE_NO  IS '手机号';
COMMENT ON COLUMN T_DBDLP_TEST_BIGTABLE.EMAIL      IS '电子邮箱';
COMMENT ON COLUMN T_DBDLP_TEST_BIGTABLE.IPV4_ADDR  IS 'IPv4地址';
COMMENT ON COLUMN T_DBDLP_TEST_BIGTABLE.ORDER_NO   IS '订单号-非敏感对照列';
COMMENT ON COLUMN T_DBDLP_TEST_BIGTABLE.CUST_LEVEL IS '客户等级-非敏感对照列';

INSERT INTO T_DBDLP_TEST_BIGTABLE (ID, MOBILE_NO, EMAIL, IPV4_ADDR, ORDER_NO, CUST_LEVEL)
SELECT LEVEL,
       '139' || LPAD(TO_CHAR(10000000 + LEVEL), 8, '0'),
       'user' || TO_CHAR(LEVEL) || '@example.com',
       '10.0.' || TO_CHAR(TRUNC(LEVEL / 256)) || '.' || TO_CHAR(MOD(LEVEL, 256)),
       'SO2026' || LPAD(TO_CHAR(LEVEL), 10, '0'),
       MOD(LEVEL, 3) + 1
FROM DUAL
CONNECT BY LEVEL <= 1000;

COMMIT;

-- ============================================================================
-- 9. 数据装载校验（执行后应显示每张表的实际行数）
-- ============================================================================
SELECT 'T_DBDLP_TEST_CUSTOMER' AS TABLE_NAME, COUNT(*) AS ROW_COUNT FROM T_DBDLP_TEST_CUSTOMER
UNION ALL SELECT 'T_DBDLP_TEST_PERSON',    COUNT(*) FROM T_DBDLP_TEST_PERSON
UNION ALL SELECT 'T_DBDLP_TEST_ENTERPRISE',COUNT(*) FROM T_DBDLP_TEST_ENTERPRISE
UNION ALL SELECT 'T_DBDLP_TEST_NETWORK',   COUNT(*) FROM T_DBDLP_TEST_NETWORK
UNION ALL SELECT 'T_DBDLP_TEST_VEHICLE',   COUNT(*) FROM T_DBDLP_TEST_VEHICLE
UNION ALL SELECT 'T_DBDLP_TEST_TEXT_LOB',  COUNT(*) FROM T_DBDLP_TEST_TEXT_LOB
UNION ALL SELECT 'T_DBDLP_TEST_NEGATIVE',  COUNT(*) FROM T_DBDLP_TEST_NEGATIVE
UNION ALL SELECT 'T_DBDLP_TEST_BIGTABLE',  COUNT(*) FROM T_DBDLP_TEST_BIGTABLE;

EXIT;
