import { defineConfig, loadEnv } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const previewHost = env.VITE_PREVIEW_HOST;

  const server = {
    host: '0.0.0.0',
    allowedHosts: ['localhost', '127.0.0.1', ...(previewHost ? [previewHost] : [])],
    proxy: {
      '/ca-cert': 'http://127.0.0.1:38081',
      '/export-map': 'http://127.0.0.1:38081',
      '/vault': 'http://127.0.0.1:38081',
      '/discovered': 'http://127.0.0.1:38081',
      '/generate-sdk': 'http://127.0.0.1:38081',
      '/sessions': 'http://127.0.0.1:38081',
      '/sessions/add-target': 'http://127.0.0.1:38081',
      '/sessions/switch': 'http://127.0.0.1:38081',
      '/sessions/delete': 'http://127.0.0.1:38081',
    },
  };

  if (previewHost) {
    server.hmr = {
      host: previewHost,
      protocol: 'wss',
      clientPort: 443,
    };
  }

  return {
    server,
    plugins: [
      VitePWA({
        registerType: 'autoUpdate',
        includeAssets: ['favicon.jpg', 'pwa-192x192.png', 'pwa-512x512.png'],
        manifest: {
          name: 'ShadowSchema Dashboard',
          short_name: 'ShadowSchema',
          description: 'Advanced API MITM Mapping Framework',
          theme_color: '#050810',
          background_color: '#050810',
          display: 'standalone',
          icons: [
            {
              src: 'pwa-192x192.png',
              sizes: '192x192',
              type: 'image/png'
            },
            {
              src: 'pwa-512x512.png',
              sizes: '512x512',
              type: 'image/png',
              purpose: 'any maskable'
            }
          ]
        }
      })
    ]
  };
});