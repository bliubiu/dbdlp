"""
针对性验证：批量导入 data.errors 在 code=0（部分成功）与 code=2001（整批失败）
两个分支都能在 P04 界面逐行回显。

被测对象：web/out 静态产物（Next.js 16 output:'export'）
服务拓扑：单端口同源，静态 + mock /api/v1，复刻 Go handleStatic 回落逻辑
交互路径：CDP DOM.setFileInputFiles 选择文件 → 点击「提交导入」→ 读 document.body.innerText

用法：python scripts/verify_import_errors.py
"""

import http.server
import json
import os
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

PARTIAL = {
    'total': 3, 'succeeded': 1, 'failed': 2,
    'errors': [
        {'row': 2, 'message': '连接测试未通过：主机不可达，请检查地址与端口；该行未入库'},
        {'row': 3, 'name': 'fin_dm_test', 'message': '实例标识已存在（含已删除记录），请更换名称；该行未入库'},
    ],
}
ALL_FAILED = {
    'total': 3, 'succeeded': 0, 'failed': 3,
    'errors': [
        {'row': 1, 'name': 'hr_a', 'message': '只读账号缺失，请补齐后重试；该行未入库'},
        {'row': 2, 'message': '连接测试未通过：主机不可达，请检查地址与端口；该行未入库'},
        {'index': 3, 'name': 'hr_c', 'error': '端口不合法，需为 1-65535；该行未入库'},
    ],
}


def free_port():
    s = socket.socket()
    s.bind(('127.0.0.1', 0))
    p = s.getsockname()[1]
    s.close()
    return p


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'

    def log_message(self, fmt, *args):
        pass

    def _json(self, code, data, message, status=200):
        body = json.dumps({'code': code, 'data': data, 'message': message}).encode('utf-8')
        self.send_response(status)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _static(self):
        rel = self.path.split('?')[0].lstrip('/')
        if rel == '':
            rel = 'index.html'
        target = os.path.abspath(os.path.join(OUT_DIR, *rel.split('/')))
        if not (target == OUT_DIR or target.startswith(OUT_DIR + os.sep)):
            self.send_error(404)
            return
        if not os.path.exists(target):
            target = os.path.join(OUT_DIR, 'index.html')
        if os.path.isdir(target):
            idx = os.path.join(target, 'index.html')
            target = idx if os.path.exists(idx) else os.path.join(OUT_DIR, 'index.html')
        ctype = 'text/html; charset=utf-8'
        if target.endswith('.js'):
            ctype = 'application/javascript; charset=utf-8'
        elif target.endswith('.css'):
            ctype = 'text/css; charset=utf-8'
        with open(target, 'rb') as fh:
            body = fh.read()
        self.send_response(200)
        self.send_header('Content-Type', ctype)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if not self.path.startswith('/api/v1/'):
            self._static()
            return
        if not (self.headers.get('Authorization') or '').startswith('Bearer '):
            self._json(1002, None, '登录状态已失效，请重新登录')
            return
        path = self.path.split('?')[0]
        if path == '/api/v1/auth/me':
            self._json(0, {'userId': 'u-1', 'username': 'admin', 'role': '管理员'}, '')
            return
        self._json(0, {'items': [], 'total': 0, 'page': 1, 'limit': 20, 'hasMore': False}, '')

    def do_POST(self):
        length = int(self.headers.get('Content-Length') or 0)
        raw = self.rfile.read(length) if length else b''
        if self.path.split('?')[0] != '/api/v1/datasources/import':
            self._json(5000, None, '系统内部错误，请联系管理员')
            return
        # 文件名含 allfail -> 整批失败（code=2001 / HTTP 422）；否则部分成功
        if b'allfail' in raw:
            self._json(2001, ALL_FAILED,
                       '批量导入完成：成功 0 行、失败 3 行，失败行未入库，可导出错误清单修正后重试',
                       status=422)
        else:
            self._json(0, PARTIAL,
                       '批量导入完成：成功 1 行、失败 2 行，失败行未入库，可导出错误清单修正后重试')


class Chrome:
    def __init__(self, port):
        self.port = port
        self.proc = None
        self.ws = None
        self.msg_id = 0
        self.profile = None

    def start(self):
        self.profile = tempfile.mkdtemp(prefix='cdp-import-')
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
        res = self.cmd('Runtime.evaluate',
                       {'expression': expr, 'returnByValue': True}, session=session)
        if 'exceptionDetails' in res:
            return None
        return res.get('result', {}).get('value')

    def stop(self):
        try:
            if self.ws:
                self.ws.close()
        except Exception:
            pass
        if self.proc:
            self.proc.terminate()


results = []


def check(name, ok, detail=''):
    results.append((name, ok, detail))
    print('%-52s %-5s %s' % (name, 'PASS' if ok else 'FAIL', detail))


def main():
    port, cdp_port = free_port(), free_port()
    srv = http.server.ThreadingHTTPServer(('127.0.0.1', port), Handler)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    base = 'http://127.0.0.1:%d' % port
    print('[server] 静态 + mock API 同端口 %s' % base)

    tmp = tempfile.mkdtemp(prefix='dbdlp-import-')
    partial_file = os.path.join(tmp, 'partial.yaml')
    allfail_file = os.path.join(tmp, 'allfail.yaml')
    for path in (partial_file, allfail_file):
        with open(path, 'w', encoding='utf-8') as fh:
            fh.write('dataSources:\n  - name: demo\n    host: 127.0.0.1\n')

    chrome = Chrome(cdp_port)
    chrome.start()
    session = chrome.attach()
    chrome.cmd('Page.enable', {}, session=session)
    chrome.cmd('Runtime.enable', {}, session=session)
    chrome.cmd('DOM.enable', {}, session=session)
    chrome.evaluate(session, "sessionStorage.setItem('dbdlp.session.token','verify-token')")

    def run_case(label, file_path, expects):
        chrome.cmd('Page.navigate', {'url': base + '/datasources/import/'}, session=session)
        time.sleep(1.6)
        # sessionStorage 是按源隔离的：必须先落到站点源再写入令牌
        chrome.evaluate(session, "sessionStorage.setItem('dbdlp.session.token','verify-token')")
        chrome.cmd('Page.navigate', {'url': base + '/datasources/import/'}, session=session)
        time.sleep(1.8)
        root = chrome.cmd('DOM.getDocument', {}, session=session)
        node = chrome.cmd('DOM.querySelector',
                          {'nodeId': root['root']['nodeId'], 'selector': '#import-file'},
                          session=session)
        if not node.get('nodeId'):
            check(label, False, '未找到 #import-file')
            return ''
        chrome.cmd('DOM.setFileInputFiles', {'files': [file_path],
                                            'nodeId': node['nodeId']}, session=session)
        time.sleep(0.6)
        clicked = chrome.evaluate(session, """
        (() => {
          const b = Array.from(document.querySelectorAll('button'))
            .find(x => x.textContent.includes('提交导入'));
          if (!b) return false;
          b.click(); return true;
        })()
        """)
        if not clicked:
            check(label, False, '未找到提交按钮')
            return ''
        time.sleep(2.0)
        text = chrome.evaluate(session, 'document.body.innerText') or ''
        missing = [e for e in expects if e not in text]
        check(label, not missing, ('缺: ' + ', '.join(missing)) if missing else '')
        return text

    t1 = run_case('code=0 部分成功：失败行清单逐行回显', partial_file, [
        '失败行清单（2 行）',
        '连接测试未通过：主机不可达',
        '实例标识已存在（含已删除记录）',
        'fin_dm_test',
        '部分成功',
    ])

    t2 = run_case('code=2001 整批失败：data.errors 不丢失', allfail_file, [
        '全部未入库',
        '失败行清单（3 行）',
        '只读账号缺失',
        '连接测试未通过：主机不可达',
        '端口不合法',
        'hr_c',
        '成功 0 行、失败 3 行',
    ])

    chrome.stop()
    srv.shutdown()

    print()
    print('=' * 92)
    failed = [r for r in results if not r[1]]
    print('IMPORT ERRORS VERIFY: %d passed / %d failed' % (len(results) - len(failed), len(failed)))
    for name, _, detail in failed:
        print('  FAIL: %s  %s' % (name, detail))
    if failed:
        print('  --- code=0 页面文本片段 ---')
        print(t1[:700])
        print('  --- code=2001 页面文本片段 ---')
        print(t2[:700])
    sys.exit(1 if failed else 0)


if __name__ == '__main__':
    main()
