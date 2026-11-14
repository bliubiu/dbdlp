"""
用无头 Chrome 逐页验证 web/out 静态产物的客户端渲染结果。
每个路由使用独立 user-data-dir，避免并发实例冲突。
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile

CHROME = r'C:\Program Files\Google\Chrome\Application\chrome.exe'
BASE = sys.argv[1] if len(sys.argv) > 1 else 'http://127.0.0.1:8800'

ROUTES = [
    '/', '/login/', '/datasources/', '/datasources/new/', '/datasources/import/',
    '/tasks/', '/tasks/sample/', '/findings/', '/findings/sample/',
    '/reports/', '/reports/sample/', '/rules/', '/config/', '/datasources/sample/edit/',
]


def probe(route):
    profile = tempfile.mkdtemp(prefix='cr-')
    try:
        proc = subprocess.run(
            [CHROME, '--headless=new', '--disable-gpu', '--no-sandbox',
             '--disable-dev-shm-usage', '--no-first-run', '--no-default-browser-check',
             f'--user-data-dir={profile}', '--virtual-time-budget=6000',
             '--dump-dom', BASE + route],
            capture_output=True, timeout=120,
        )
        html = proc.stdout.decode('utf-8', 'ignore')
    except Exception as exc:  # pragma: no cover
        return route, 0, 'EXC:%s' % exc, False, True
    finally:
        shutil.rmtree(profile, ignore_errors=True)

    if not html.strip():
        return route, 0, 'EMPTY', False, True

    err = ('Application error' in html or 'Unhandled Runtime' in html
           or 'Build Error' in html)
    m = re.search(r'<h1[^>]*>(.*?)</h1>', html, re.S)
    h1 = re.sub(r'<[^>]+>', '', m.group(1)).strip() if m else '-'
    guard = '登录状态已失效' in html
    return route, len(html), h1[:22], guard, err


print('%-28s %-8s %-24s %-11s %s' % ('ROUTE', 'BYTES', 'H1', 'AUTH_GUARD', 'ERROR'))
print('-' * 92)
fails = 0
for r in ROUTES:
    route, size, h1, guard, err = probe(r)
    if err or size == 0:
        fails += 1
    print('%-28s %-8d %-24s %-11s %s' % (route, size, h1, guard, err))
print('-' * 92)
print('FAILED PAGES: %d / %d' % (fails, len(ROUTES)))
