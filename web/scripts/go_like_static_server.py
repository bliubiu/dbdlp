"""
精确模拟 Go 侧 internal/interfaces/http/static.go 的 handleStatic 行为，
用于验证 web/out 静态产物在同端口托管下的真实路由命中情况。

对应 Go 逻辑：
  rel := trim(path, "/")            // 空 -> index.html
  target := staticDir/rel
  os.Stat 失败            -> serveIndex（返回根 index.html）
  target 是目录            -> 读 <dir>/index.html，不存在则 serveIndex
  否则                    -> ServeFile(target)
"""

import http.server
import os
import posixpath
import sys

STATIC_DIR = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else 'out')
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 8800


class GoLikeHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass

    def do_GET(self):
        rel = self.path.split('?', 1)[0].split('#', 1)[0].lstrip('/')
        if rel == '':
            rel = 'index.html'
        target = os.path.join(STATIC_DIR, *rel.split('/'))
        target = os.path.abspath(target)
        root = STATIC_DIR
        if target != root and not target.startswith(root + os.sep):
            self.send_error(404)
            return
        if not os.path.exists(target):
            self.serve_index()
            return
        if os.path.isdir(target):
            index = os.path.join(target, 'index.html')
            if not os.path.exists(index):
                self.serve_index()
                return
            self.send_file(index)
            return
        self.send_file(target)

    def do_HEAD(self):
        self.do_GET()

    def serve_index(self):
        index = os.path.join(STATIC_DIR, 'index.html')
        if not os.path.exists(index):
            self.send_error(404)
            return
        self.send_file(index)

    def send_file(self, path):
        ctype = 'text/html; charset=utf-8'
        if path.endswith('.js'):
            ctype = 'application/javascript; charset=utf-8'
        elif path.endswith('.css'):
            ctype = 'text/css; charset=utf-8'
        elif path.endswith('.txt'):
            ctype = 'text/plain; charset=utf-8'
        elif path.endswith('.json'):
            ctype = 'application/json'
        with open(path, 'rb') as fh:
            body = fh.read()
        self.send_response(200)
        self.send_header('Content-Type', ctype)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == '__main__':
    server = http.server.ThreadingHTTPServer(('127.0.0.1', PORT), GoLikeHandler)
    print(f'Go-like static server on http://127.0.0.1:{PORT} serving {STATIC_DIR}', flush=True)
    server.serve_forever()
