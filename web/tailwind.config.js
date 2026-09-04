/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        // beautifului.dev dark tokens (scraped, oklch)
        page: 'var(--page)',
        surface: 'var(--surface)',
        panel: 'var(--panel)',
        field: 'var(--field)',
        control: 'var(--control)',
        ink: 'var(--ink)',
        'ink-2': 'var(--ink-2)',
        'ink-3': 'var(--ink-3)',
        line: 'var(--line)',
        'line-strong': 'var(--line-strong)',
        accent: 'var(--accent)',
        'accent-ink': 'var(--accent-ink)',
        ok: 'var(--ok)',
        bad: 'var(--bad)',
        warn: 'var(--warn)',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['Geist Mono', 'JetBrains Mono', 'SF Mono', 'monospace'],
      },
      borderRadius: { card: '10px', control: '8px', chip: '6px', xl2: '10px' },
    },
  },
  plugins: [],
}
