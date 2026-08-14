import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Çıktı Go binary'sine gömülür (go:embed) — embed deseni paket diziniyle
// sınırlı olduğundan çıktı internal/webui/dist'e yazılır.
export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 6000,
  },
})
