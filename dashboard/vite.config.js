import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
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
})
