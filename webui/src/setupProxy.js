const { createProxyMiddleware } = require('http-proxy-middleware');

module.exports = function(app) {
  const defaultTarget = 'http://localhost:8080';
  const target = process.env.WEBUI_PROXY_TARGET || process.env.REACT_APP_WEBUI_PROXY_TARGET || defaultTarget;
  const secure = process.env.WEBUI_PROXY_SECURE === 'false' ? false : true;

  app.get('/config.js', (req, res) => {
    const apiBase = process.env.WEBUI_API_BASE || process.env.REACT_APP_API_BASE || '';
    res.set('Content-Type', 'application/javascript');
    res.send(`window.__API_BASE = ${JSON.stringify(apiBase)};\n`);
  });

  const streamingProxyConfig = {
    target,
    changeOrigin: true,
    secure,
    logLevel: 'debug',
    ws: false,
    timeout: 0,
    proxyTimeout: 0,
    onProxyReq: (proxyReq) => {
      const token = process.env.DEV_BEARER_TOKEN || process.env.REACT_APP_DEV_BEARER_TOKEN;
      if (token) proxyReq.setHeader('Authorization', `Bearer ${token}`);
      console.log('Proxy streaming request to:', proxyReq.path);
    },
    onProxyRes: (proxyRes, req, res) => {
      console.log('Proxy streaming response:', proxyRes.statusCode);
      // Ensure streaming headers are set
      proxyRes.headers['Access-Control-Allow-Origin'] = '*';
      proxyRes.headers['Access-Control-Allow-Headers'] = 'Cache-Control';
      proxyRes.headers['Cache-Control'] = 'no-cache';
      proxyRes.headers['Connection'] = 'keep-alive';
      proxyRes.headers['Content-Type'] = 'text/event-stream';
    },
    onError: (err, req, res) => {
      console.error('Proxy streaming error:', err);
    }
  };

  app.use('/v1/builds/:name/logs/sse', createProxyMiddleware(streamingProxyConfig));

  app.use(
    '/v1',
    createProxyMiddleware({
      target,
      changeOrigin: true,
      secure,
      logLevel: 'debug',
      onProxyReq: (proxyReq) => {
        const token = process.env.DEV_BEARER_TOKEN || process.env.REACT_APP_DEV_BEARER_TOKEN;
        if (token) proxyReq.setHeader('Authorization', `Bearer ${token}`);
      }
    })
  );
};