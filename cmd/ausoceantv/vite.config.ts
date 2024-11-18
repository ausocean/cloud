import { defineConfig } from 'vite'
import { resolve } from 'path'


export default defineConfig({
    build: {
        rollupOptions: {
            input: {
                main: resolve(__dirname, 'index.html'),
                home: resolve(__dirname, 'home.html'),
                watch: resolve(__dirname, 'watch.html'),
            }
        }
    },
    server: {
        proxy: {
          "/auth": {
            target: "http://localhost:8084",
            changeOrigin: true,
          },
        },
      },
})