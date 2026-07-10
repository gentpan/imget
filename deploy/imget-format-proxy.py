#!/usr/bin/env python3
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlsplit, urlunsplit, parse_qsl, urlencode
import http.client
import re

UPSTREAM_HOST = '127.0.0.1'
UPSTREAM_PORT = 18081
LISTEN = ('127.0.0.1', 18080)
HOP_BY_HOP = {'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailers', 'transfer-encoding', 'upgrade'}
SIZE_EXT = re.compile(r'^/(\d+)[x/](\d+)\.(webp|avif)$', re.I)
MOBILE_CSS_VERSION = '202605312215-mobile'
MOBILE_CSS = r'''

/* mobile layout patch injected by imget-format-proxy */
html,
body {
  max-width: 100%;
  overflow-x: hidden;
}

.wrap,
.hero,
.section,
.code-card,
.tester-actions,
.tester-url,
.summary-bar,
.footer {
  max-width: 100%;
}

@media (max-width: 720px) {
  body {
    font-size: 15px;
    line-height: 1.7;
  }

  .wrap {
    width: 100%;
    padding: 10px 12px 34px;
  }

  .hero,
  .section {
    width: 100%;
    padding: 14px;
    margin-left: 0;
    margin-right: 0;
  }

  .hero-top {
    display: block;
  }

  .hero-brand {
    width: 100%;
    justify-content: flex-start;
  }

  .brand-wordmark {
    width: 168px;
    min-height: 58px;
  }

  .brand-wordmark::before {
    font-size: 36px;
    letter-spacing: -0.06em;
  }

  .hero-badges {
    display: grid;
    grid-template-columns: 1fr;
    justify-content: stretch;
    width: 100%;
    margin-top: 8px;
  }

  .hero-badge {
    width: 100%;
    justify-content: center;
    min-height: 34px;
    padding: 0 8px;
    font-size: 13px;
  }

  .summary-bar span {
    width: 100%;
    min-width: 0;
    flex-wrap: wrap;
  }

  .tester-grid {
    display: grid !important;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 12px;
  }

  .tester-grid > .tester-field {
    min-width: 0;
  }

  .tester-field:nth-last-child(-n + 2),
  .tester-r-field {
    grid-column: 1 / -1;
  }

  .tester-actions {
    display: grid;
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .tester-url {
    width: 100%;
    flex: none;
    min-height: 44px;
    padding: 10px 11px;
    font-size: 12px;
    line-height: 1.55;
  }

  .tester-buttons {
    display: grid;
    grid-template-columns: 1fr 1fr;
    width: 100%;
    gap: 8px;
  }

  .action-btn {
    width: 100%;
    min-width: 0;
    height: 44px;
    padding: 0 10px;
    font-size: 14px;
  }

  .code-head {
    flex-wrap: nowrap;
  }

  .code-head span {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  pre {
    white-space: pre-wrap;
    overflow-x: auto;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  table {
    max-width: 100%;
  }

  .footer-copy,
  .footer-stats,
  .footer-links {
    width: 100%;
  }
}

@media (max-width: 430px) {
  .wrap {
    padding-left: 10px;
    padding-right: 10px;
  }

  .tester-grid,
  .tester-buttons {
    grid-template-columns: 1fr;
  }

  .tester-field {
    grid-column: 1 / -1;
  }

  .section h2 {
    font-size: 19px;
  }
}
'''

class Proxy(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    server_version = 'imget'
    sys_version = ''

    def log_message(self, fmt, *args):
        return

    def _rewrite(self):
        parsed = urlsplit(self.path)
        path = parsed.path
        query = parse_qsl(parsed.query, keep_blank_values=True)
        keys = {k.lower() for k, _ in query}

        m = SIZE_EXT.match(path)
        if m:
            path = f'/{m.group(1)}/{m.group(2)}'
            if 'format' not in keys:
                query.append(('format', m.group(3).lower()))
                keys.add('format')

        if 'format' not in keys:
            accept = self.headers.get('Accept', '').lower()
            if 'image/webp' in accept:
                query.append(('format', 'webp'))
            elif 'image/avif' in accept:
                query.append(('format', 'avif'))
        return urlunsplit(('', '', path, urlencode(query, doseq=True), parsed.fragment))

    def _proxy(self):
        body = None
        length = self.headers.get('Content-Length')
        if length:
            body = self.rfile.read(int(length))

        headers = {k: v for k, v in self.headers.items() if k.lower() not in HOP_BY_HOP and k.lower() != 'host'}
        headers['Host'] = f'{UPSTREAM_HOST}:{UPSTREAM_PORT}'
        headers['X-Forwarded-Host'] = self.headers.get('Host', '')
        headers['X-Forwarded-Proto'] = 'https'

        conn = http.client.HTTPConnection(UPSTREAM_HOST, UPSTREAM_PORT, timeout=120)
        try:
            conn.request(self.command, self._rewrite(), body=body, headers=headers)
            resp = conn.getresponse()
            response_headers = resp.getheaders()
            content_type = dict((k.lower(), v) for k, v in response_headers).get('content-type', '')
            parsed_path = urlsplit(self.path).path
            should_patch_css = self.command != 'HEAD' and parsed_path == '/assets/main.min.css'
            should_patch_html = self.command != 'HEAD' and parsed_path == '/' and 'text/html' in content_type

            if should_patch_css or should_patch_html:
                content = resp.read()
                if should_patch_css:
                    content = content + MOBILE_CSS.encode('utf-8')
                else:
                    html = content.decode('utf-8', errors='replace')
                    # Keep imget's own asset version and just append the mobile
                    # suffix, so a version bump upstream (site.go assetVersion)
                    # auto-busts the mobile CSS cache too. A static version here
                    # would pin browsers to a stale patched stylesheet forever.
                    html = re.sub(
                        r'/assets/main\.min\.css\?v=([0-9A-Za-z_.]+)',
                        r'/assets/main.min.css?v=\1-mobile',
                        html,
                    )
                    content = html.encode('utf-8')

                self.send_response(resp.status, resp.reason)
                for k, v in response_headers:
                    if k.lower() not in HOP_BY_HOP and k.lower() not in {'server', 'date', 'content-length', 'etag'}:
                        self.send_header(k, v)
                self.send_header('Content-Length', str(len(content)))
                self.end_headers()
                self.wfile.write(content)
                self.wfile.flush()
                return

            has_content_length = any(k.lower() == 'content-length' for k, _ in response_headers)

            self.send_response(resp.status, resp.reason)
            for k, v in response_headers:
                if k.lower() not in HOP_BY_HOP and k.lower() not in {'server', 'date'}:
                    self.send_header(k, v)

            # Upstream chunked responses lose their Transfer-Encoding here.
            # Closing the downstream connection gives Caddy a clear EOF.
            if not has_content_length:
                self.send_header('Connection', 'close')
                self.close_connection = True

            self.end_headers()
            if self.command != 'HEAD':
                while True:
                    chunk = resp.read(1024 * 256)
                    if not chunk:
                        break
                    self.wfile.write(chunk)
                self.wfile.flush()
        finally:
            conn.close()

    def do_GET(self): self._proxy()
    def do_HEAD(self): self._proxy()
    def do_POST(self): self._proxy()
    def do_OPTIONS(self): self._proxy()

if __name__ == '__main__':
    ThreadingHTTPServer(LISTEN, Proxy).serve_forever()
