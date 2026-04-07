// Aurum — Static file server + API proxy for the dashboard.
// In production, Aurum is served by the Primarch service directly.
// This dev server provides hot-reload and API proxying.

const http = require('http');
const https = require('https');
const fs = require('fs');
const path = require('path');

const PORT = process.env.AURUM_PORT || 3000;
const PRIMARCH_URL = process.env.PRIMARCH_URL || 'http://localhost:8401';
const proxyLib = PRIMARCH_URL.startsWith('https') ? https : http;

const MIME = {
  '.html': 'text/html',
  '.css': 'text/css',
  '.js': 'application/javascript',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
};

const server = http.createServer((req, res) => {
  // Proxy API requests to Primarch
  if (req.url.startsWith('/api/') || req.url === '/health' || req.url === '/ws') {
    const proxyReq = proxyLib.request(
      `${PRIMARCH_URL}${req.url}`,
      { method: req.method, headers: { ...req.headers, host: new URL(PRIMARCH_URL).host } },
      (proxyRes) => {
        res.writeHead(proxyRes.statusCode, proxyRes.headers);
        proxyRes.pipe(res);
      }
    );
    proxyReq.on('error', () => {
      res.writeHead(502);
      res.end(JSON.stringify({ error: 'Primarch unreachable' }));
    });
    req.pipe(proxyReq);
    return;
  }

  // Serve static files from public/
  let filePath = path.join(__dirname, 'public', req.url === '/' ? 'index.html' : req.url);

  // SPA fallback — serve index.html for unknown routes
  if (!fs.existsSync(filePath)) {
    filePath = path.join(__dirname, 'public', 'index.html');
  }

  const ext = path.extname(filePath);
  res.writeHead(200, { 'Content-Type': MIME[ext] || 'text/plain' });
  fs.createReadStream(filePath).pipe(res);
});

server.listen(PORT, () => {
  console.log(`\n  STRATEGIUM dashboard: http://localhost:${PORT}`);
  console.log(`  Proxying API to: ${PRIMARCH_URL}\n`);
});
