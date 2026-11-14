"""
dbdlp 前端控制台 · 端到端冒烟验证（无头 Chrome + mock 后端，真实浏览器交互）

被测对象：web/out 静态产物（Next.js 16 output: 'export'，trailingSlash: true）
服务拓扑：单端口同源 —— 静态资源与 /api/v1/* 由同一 HTTP 服务提供，
          完全复刻 Go 侧 handleStatic 的「/api/v1 优先、未知路径回落根 index.html」逻辑。

验证项：
  1. 登录失败回显后端中文 message（不暴露英文/堆栈）
  2. 登录成功跳转并把 token 写入 sessionStorage
  3. 13 个页面在真实浏览器中拿到真实数据并渲染
  4. 请求携带 Authorization: Bearer
  5. 令牌失效后出现中文「登录状态已失效，请重新登录」

用法：python scripts/e2e_smoke.py
"""

import http.server
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

import websocket

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
OUT_DIR = os.path.join(ROOT, 'out')
CHROME = r'C:\Program Files\Google\Chrome\Application\chrome.exe'

STATE = {'tokens_seen': set(), 'api_paths': []}


def free_port():
    s = socket.socket()
    s.bind(('127.0.0.1', 0))
    port = s.getsockname()[1]
    s.close()
    return port


# --------------------------------------------------------------- mock 数据

DASHBOARD = {
    'riskCounts': {'high': 37, 'medium': 118, 'low': 64, 'pending': 21},
    'typeCounts': {'身份证号': 44, '手机号': 39, '银行卡号': 17, '姓名': 88, '地址': 33},
    'schemaCounts': {'HR': 96, 'FIN': 78, 'OPS': 41},
    'tableCount': 215, 'columnCount': 4820, 'hitColumnCount': 240,
}

DATASOURCES = [
    {'id': 'ds-001', 'name': 'hr_dm_prod', 'dialect': 'dm', 'host': '*.*.10.3', 'port': 5236,
     'serviceName': 'HRDB', 'username': 'scanner_ro', 'password': '******',
     'includeSchemas': ['HR'], 'excludeTables': [], 'excludeColumns': [],
     'connectionState': 'ok', 'lastTestAt': '2026-11-13T09:20:00Z', 'lastScanAt': '2026-11-13T09:40:00Z'},
    {'id': 'ds-002', 'name': 'fin_dm_test', 'dialect': 'dm', 'host': '127.0.0.1', 'port': 5236,
     'serviceName': 'FINDB', 'username': 'scanner_ro', 'password': '******',
     'includeSchemas': ['FIN'], 'excludeTables': [], 'excludeColumns': [],
     'connectionState': 'failed', 'lastTestAt': '2026-11-13T08:10:00Z'},
]

TASKS = [
    {'id': 'task-20261113-01', 'dataSourceId': 'ds-001', 'dataSourceName': 'hr_dm_prod',
     'status': 'running', 'ruleVersion': '2026.11.13', 'sampleRows': 20, 'batchSize': 200,
     'concurrency': 2, 'lowPeakWindow': '', 'createdAt': '2026-11-13T09:40:00Z',
     'summary': {'totalTables': 120, 'doneTables': 46, 'failedTables': 2, 'skippedTables': 1,
                 'totalColumns': 2600, 'hitColumns': 118}},
    {'id': 'task-20261112-02', 'dataSourceId': 'ds-002', 'dataSourceName': 'fin_dm_test',
     'status': 'partial', 'ruleVersion': '2026.11.12', 'sampleRows': 20,
     'createdAt': '2026-11-12T22:10:00Z',
     'summary': {'totalTables': 95, 'doneTables': 93, 'failedTables': 2, 'skippedTables': 0,
                 'totalColumns': 2220, 'hitColumns': 122,
                 'failedObjects': [{'id': 'fo-1', 'schemaName': 'FIN', 'tableName': 'T_PAY',
                                    'reason': '连接测试未通过：主机不可达'}]}},
]

FINDINGS = [
    {'id': 'f-001', 'instanceId': 'ds-001', 'schemaName': 'HR', 'tableName': 'T_PERSON',
     'columnName': 'ID_CARD', 'columnComment': '身份证号', 'dataType': 'VARCHAR(64)',
     'sensitiveTypes': ['身份证号'], 'hitRatio': 0.93, 'sampleHit': 186, 'sampleTotal': 200,
     'verifyPassed': True, 'confidence': 'high', 'storageForm': 'plaintext',
     'riskLevel': 'high', 'reviewStatus': 'pending', 'sampleMasked': '1101**********1234',
     'updatedAt': '2026-11-13T09:52:00Z'},
    {'id': 'f-002', 'instanceId': 'ds-001', 'schemaName': 'HR', 'tableName': 'T_PERSON',
     'columnName': 'MOBILE', 'columnComment': '手机号', 'dataType': 'VARCHAR(32)',
     'sensitiveTypes': ['手机号'], 'hitRatio': 0.61, 'sampleHit': 122, 'sampleTotal': 200,
     'verifyPassed': False, 'confidence': 'medium', 'storageForm': 'encrypted',
     'riskLevel': 'medium', 'reviewStatus': 'confirmed', 'sampleMasked': '138****1234',
     'updatedAt': '2026-11-13T09:52:00Z'},
]

REPORTS = [
    {'id': 'r-001', 'taskId': 'task-20261113-01', 'taskName': 'hr_dm_prod', 'format': 'excel',
     'status': 'success', 'filePath': 'reports/task-20261113-01.xlsx', 'fileSize': 28416,
     'createdAt': '2026-11-13T10:02:00Z'},
]

RULES = {
    'version': '2026.11.13',
    'rules': [
        {'id': 'r-idcard', 'name': '身份证号识别', 'sensitiveType': '身份证号',
         'pattern': '18 位身份证号，末位允许 X', 'threshold': 0, 'enabled': True,
         'validateStatus': 'valid', 'description': '18 位身份证号，末位允许 X'},
        {'id': 'r-mobile', 'name': '手机号识别', 'sensitiveType': '手机号',
         'pattern': '11 位手机号', 'threshold': 0, 'enabled': True, 'validateStatus': 'valid'},
    ],
}

CONFIG = {'logLevel': 'info', 'sampleRows': 20, 'storageDriver': 'sqlite',
          'cryptoAlgorithm': 'AES-GCM-256', 'retentionDays': 180}

AUDIT = [{'id': 'a-1', 'actor': 'admin', 'action': '登录', 'target': 'session', 'ip': '*.*.1.24',
          'detail': '会话建立', 'createdAt': '2026-11-13T09:00:00Z'}]


def page(items, total=None):
    return {'items': items, 'total': total if total is not None else len(items),
            'page': 1, 'limit': 50, 'hasMore': False}


# ------------------------------------------------------------ 同端口服务

class CombinedHandler(http.server.BaseHTTPRequestHandler):
    """同时提供静态资源与 /api/v1/*，行为对齐 Go 侧 handleStatic。"""

    protocol_version = 'HTTP/1.1'

    def log_message(self, fmt, *args):
        pass

    # ---------------- 基础工具

    def _json(self, code, data, message=''):
        body = json.dumps({'code': code, 'data': data, 'message': message}).encode('utf-8')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _file(self, path):
        if not os.path.exists(path):
            self.send_error(404)
            return
        ctype = 'text/html; charset=utf-8'
        if path.endswith('.js'):
            ctype = 'application/javascript; charset=utf-8'
        elif path.endswith('.css'):
            ctype = 'text/css; charset=utf-8'
        elif path.endswith('.txt'):
            ctype = 'text/plain; charset=utf-8'
        with open(path, 'rb') as fh:
            body = fh.read()
        self.send_response(200)
        self.send_header('Content-Type', ctype)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _static(self):
        rel = self.path.split('?')[0].split('#')[0].lstrip('/')
        if rel == '':
            rel = 'index.html'
        target = os.path.abspath(os.path.join(OUT_DIR, *rel.split('/')))
        if not (target == OUT_DIR or target.startswith(OUT_DIR + os.sep)):
            self.send_error(404)
            return
        if not os.path.exists(target):
            self._file(os.path.join(OUT_DIR, 'index.html'))
            return
        if os.path.isdir(target):
            index = os.path.join(target, 'index.html')
            self._file(index if os.path.exists(index) else os.path.join(OUT_DIR, 'index.html'))
            return
        self._file(target)

    # ---------------- API

    def _record(self):
        path = self.path.split('?')[0]
        STATE['api_paths'].append(path)
        auth = self.headers.get('Authorization') or ''
        if auth.startswith('Bearer '):
            STATE['tokens_seen'].add(auth[7:])
            return True
        return False

    def do_GET(self):
        if not self.path.startswith('/api/v1/'):
            self._static()
            return
        path = self.path.split('?')[0]
        if not self._record():
            self._json(1002, None, '登录状态已失效，请重新登录')
            return
        if path == '/api/v1/auth/me':
            return self._json(0, {'userId': 'u-1', 'username': 'admin', 'role': '管理员'})
        if path == '/api/v1/dashboard/summary':
            return self._json(0, DASHBOARD)
        if path == '/api/v1/config':
            return self._json(0, CONFIG)
        if path == '/api/v1/rules':
            return self._json(0, RULES)
        if path == '/api/v1/audit-logs':
            return self._json(0, page(AUDIT, 1))
        if path == '/api/v1/rules/snapshots':
            return self._json(0, page([
                {'version': '2026.11.13', 'createdAt': '2026-11-13T00:00:00Z', 'ruleCount': 42},
                {'version': '2026.11.12', 'createdAt': '2026-11-12T00:00:00Z', 'ruleCount': 41}], 2))
        if path == '/api/v1/datasources':
            return self._json(0, page(DATASOURCES, 2))
        if path == '/api/v1/tasks':
            return self._json(0, page(TASKS, 2))
        if path == '/api/v1/findings':
            return self._json(0, page(FINDINGS, 2))
        if path == '/api/v1/reports':
            return self._json(0, page(REPORTS, 1))
        if re.match(r'^/api/v1/tasks/[^/]+/progress$', path):
            return self._json(0, {'status': 'running', 'doneTables': 46, 'totalTables': 120,
                                  'doneColumns': 980, 'totalColumns': 2600, 'failedCount': 2})
        if re.match(r'^/api/v1/tasks/[^/]+$', path):
            return self._json(0, TASKS[0])
        if re.match(r'^/api/v1/findings/[^/]+/hits$', path):
            return self._json(0, page([{'id': 'h-1', 'findingId': 'f-001',
                                        'sampleMasked': '1101**********1234',
                                        'ruleId': 'r-idcard', 'ruleName': '身份证号识别',
                                        'matchedAt': '2026-11-13T09:52:00Z'}], 1))
        if re.match(r'^/api/v1/findings/[^/]+$', path):
            return self._json(0, FINDINGS[0])
        if path == '/api/v1/reviews':
            return self._json(0, page([], 0))
        return self._json(5000, None, '系统内部错误，请联系管理员')

    def do_POST(self):
        if not self.path.startswith('/api/v1/'):
            self.send_error(405)
            return
        length = int(self.headers.get('Content-Length') or 0)
        raw = self.rfile.read(length) if length else b''
        path = self.path.split('?')[0]
        self._record()

        if path == '/api/v1/auth/login':
            try:
                payload = json.loads(raw.decode('utf-8'))
            except Exception:
                payload = {}
            if payload.get('username') != 'admin' or payload.get('password') != 'correct-pass':
                return self._json(1001, None, '账号或密码不正确，请重新输入')
            return self._json(0, {'token': 'mock-session-token-abc123',
                                  'expiresAt': '2026-11-13T18:00:00Z',
                                  'displayName': '系统管理员'})
        if re.match(r'^/api/v1/datasources/[^/]+/test$', path):
            return self._json(0, {'ok': True, 'latencyMs': 42, 'message': '连接成功'})
        if path == '/api/v1/reviews':
            return self._json(0, {'id': 'review-1'})
        if path == '/api/v1/datasources':
            return self._json(0, page(DATASOURCES, 2))
        if path == '/api/v1/tasks':
            return self._json(0, page(TASKS, 2))
        if path == '/api/v1/reports':
            return self._json(0, [{'format': 'excel', 'artifactId': 'r-001',
                                   'filePath': 'reports/x.xlsx'}])
        return self._json(5000, None, '系统内部错误，请联系管理员')


# --------------------------------------------------------------- CDP 驱动

class Chrome:
    def __init__(self, port):
        self.port = port
        self.proc = None
        self.ws = None
        self.msg_id = 0
        self.profile = None

    def start(self):
        self.profile = tempfile.mkdtemp(prefix='cdp-')
        self.proc = subprocess.Popen(
            [CHROME, '--headless=new', '--disable-gpu', '--no-sandbox',
             '--disable-dev-shm-usage', '--no-first-run', '--no-default-browser-check',
             '--disable-extensions', f'--user-data-dir={self.profile}',
             f'--remote-debugging-port={self.port}', 'about:blank'],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        ws_url = None
        for _ in range(80):
            try:
                with urllib.request.urlopen(
                        f'http://127.0.0.1:{self.port}/json/version', timeout=2) as r:
                    ws_url = json.loads(r.read().decode())['webSocketDebuggerUrl']
                break
            except Exception:
                time.sleep(0.5)
        if not ws_url:
            raise RuntimeError('Chrome CDP 未就绪')
        self.ws = websocket.create_connection(ws_url, timeout=60)

    def cmd(self, method, params=None, session=None):
        self.msg_id += 1
        mid = self.msg_id
        payload = {'id': mid, 'method': method, 'params': params or {}}
        if session:
            payload['sessionId'] = session
        self.ws.send(json.dumps(payload))
        while True:
            msg = json.loads(self.ws.recv())
            if msg.get('id') == mid:
                return msg.get('result', {})

    def attach(self):
        res = self.cmd('Target.createTarget', {'url': 'about:blank'})
        res = self.cmd('Target.attachToTarget',
                       {'targetId': res['targetId'], 'flatten': True})
        return res['sessionId']

    def evaluate(self, session, expr):
        # cmd() 已返回消息体的 result 字段；Runtime.evaluate 的结果在其 .result 里
        res = self.cmd('Runtime.evaluate',
                       {'expression': expr, 'returnByValue': True}, session=session)
        if 'exceptionDetails' in res:
            return None
        return res.get('result', {}).get('value')

    def navigate(self, session, url, wait=1.4):
        self.cmd('Page.navigate', {'url': url}, session=session)
        time.sleep(wait)

    def stop(self):
        try:
            if self.ws:
                self.ws.close()
        except Exception:
            pass
        if self.proc:
            self.proc.terminate()
        shutil.rmtree(self.profile, ignore_errors=True)


FILL_LOGIN = """
(() => {
  const setVal = (el, v) => {
    const desc = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), 'value');
    desc.set.call(el, v);
    el.dispatchEvent(new Event('input', {bubbles: true}));
  };
  const inputs = document.querySelectorAll('input');
  if (inputs.length < 2) return 'NO_INPUT';
  setVal(inputs[0], USER);
  setVal(inputs[1], PASS);
  const form = document.querySelector('form');
  if (form) { form.requestSubmit(); return 'SUBMITTED'; }
  const btn = Array.from(document.querySelectorAll('button'))
      .find(b => b.textContent.includes('登录'));
  if (btn) { btn.click(); return 'CLICKED'; }
  return 'NO_TRIGGER';
})()
"""


def fill_and_submit(chrome, session, user, pwd):
    expr = FILL_LOGIN.replace('USER', json.dumps(user)).replace('PASS', json.dumps(pwd))
    return chrome.evaluate(session, expr)


# ------------------------------------------------------------------ 主流程

def main():
    port = free_port()
    cdp_port = free_port()

    srv = http.server.ThreadingHTTPServer(('127.0.0.1', port), CombinedHandler)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    base = 'http://127.0.0.1:%d' % port
    print('[server] 静态 + mock API 同端口 %s' % base)

    chrome = Chrome(cdp_port)
    chrome.start()
    session = chrome.attach()
    chrome.cmd('Page.enable', {}, session=session)
    chrome.cmd('Runtime.enable', {}, session=session)

    results = []

    def check(name, ok, detail=''):
        results.append((name, ok, detail))
        print('%-46s %-5s %s' % (name, 'PASS' if ok else 'FAIL', detail))

    # 1) 登录失败：回显后端中文 message
    chrome.navigate(session, base + '/login/')
    fill_and_submit(chrome, session, 'admin', 'wrong-pass')
    time.sleep(2.0)
    text = chrome.evaluate(session, 'document.body.innerText') or ''
    check('P13 登录失败回显中文 message', '账号或密码不正确' in text,
          '' if '账号或密码不正确' in text else text[:100])

    # 2) 登录成功：跳转 + token 入 sessionStorage
    chrome.navigate(session, base + '/login/')
    fill_and_submit(chrome, session, 'admin', 'correct-pass')
    time.sleep(2.5)
    path = chrome.evaluate(session, 'location.pathname')
    check('P13 登录成功跳转', bool(path) and path != '/login/', 'pathname=%s' % path)
    token = chrome.evaluate(session, "sessionStorage.getItem('dbdlp.session.token')")
    check('token 写入 sessionStorage', bool(token), 'token=%s' % (token[:14] + '..' if token else 'NONE'))
    local = chrome.evaluate(session, "String(localStorage.getItem('dbdlp.session.token'))")
    check('token 不落 localStorage', local in (None, 'null'), 'localStorage=%s' % local)

    # 3) 逐页验证真实数据渲染
    pages = [
        ('/', ['总览工作台', '数据源总数', '连接异常'], 'P01 总览工作台'),
        ('/datasources/', ['hr_dm_prod', '*.*.10.3', '连接正常'], 'P02 数据源管理'),
        ('/datasources/new/', ['实例标识', '连接测试'], 'P03 新建数据源'),
        ('/datasources/import/', ['批量导入', 'Excel'], 'P04 批量导入'),
        ('/tasks/', ['task-20261113-01', '运行中'], 'P05 扫描任务列表'),
        ('/tasks/task-20261113-01/', ['task-20261113-01', '扫描进度'], 'P06 任务详情（真实 id）'),
        ('/findings/', ['ID_CARD', '1101**********1234', '身份证号'], 'P07 结果台账'),
        ('/findings/f-001/', ['ID_CARD', '身份证号'], 'P08 字段明细（真实 id）'),
        ('/reports/', ['Excel 台账', 'hr_dm_prod'], 'P09 报告中心'),
        ('/reports/r-001/', ['扫描概况', '附录'], 'P10 报告预览（真实 id）'),
        ('/rules/', ['身份证号识别', '2026.11.13'], 'P11 规则管理'),
        ('/config/', ['生效配置', 'AES-GCM-256'], 'P12 系统设置'),
    ]
    for route, expects, label in pages:
        chrome.navigate(session, base + route, wait=1.8)
        text = chrome.evaluate(session, 'document.body.innerText') or ''
        missing = [e for e in expects if e not in text]
        check(label, not missing, ('缺: ' + ', '.join(missing)) if missing else '')

    # 4) 鉴权头
    check('请求携带 Bearer token',
          'mock-session-token-abc123' in STATE['tokens_seen'],
          'tokens=%s' % sorted(STATE['tokens_seen'])[:1])

    # 5) 明文样例未泄漏（掩码约束）
    leaked = [p for p in STATE['api_paths'] if 'raw' in p.lower()]
    check('接口路径未出现明文字段（raw_value 等）', not leaked, str(leaked[:3]))

    # 6) 令牌失效 -> 中文提示
    chrome.evaluate(session, "sessionStorage.removeItem('dbdlp.session.token')")
    chrome.navigate(session, base + '/findings/', wait=2.2)
    text = chrome.evaluate(session, 'document.body.innerText') or ''
    check('令牌失效显示中文失效提示', '登录状态已失效' in text,
          '' if '登录状态已失效' in text else text[:100])

    chrome.stop()
    srv.shutdown()

    print()
    print('=' * 92)
    failed = [r for r in results if not r[1]]
    print('E2E RESULT: %d passed / %d failed' % (len(results) - len(failed), len(failed)))
    for name, _, detail in failed:
        print('  FAIL: %s  %s' % (name, detail))
    sys.exit(1 if failed else 0)


if __name__ == '__main__':
    main()
