import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    environment: 'node',
    coverage: {
      provider: 'v8',
      reporter: ['text-summary', 'lcov'],
      // Only files that tests actually reach. Reporting 32 untested sources as
      // 0% buries the signal for the ones under test; the ratchet below is what
      // grows this list.
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.ts', 'src/main.tsx', 'src/vite-env.d.ts'],
    },
  },
})
