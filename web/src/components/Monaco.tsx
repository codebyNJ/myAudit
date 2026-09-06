import Editor, { loader, type OnMount } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'

;(self as unknown as { MonacoEnvironment: monaco.Environment }).MonacoEnvironment = {
  getWorker() {
    return new Worker(URL.createObjectURL(new Blob(['self.onmessage=()=>{}'], { type: 'application/javascript' })))
  },
}
loader.config({ monaco })

let themed = false
function ensureTheme(m: typeof monaco) {
  if (themed) return
  themed = true
  m.editor.defineTheme('cursor-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: 'comment', foreground: '52525b', fontStyle: 'italic' },
      { token: 'keyword', foreground: 'f43f5e' },
      { token: 'string', foreground: '7dd3fc' },
      { token: 'number', foreground: 'f59e0b' },
      { token: 'type', foreground: '38bdf8' },
      { token: 'function', foreground: 'c084fc' },
      { token: 'variable', foreground: 'e4e4e7' },
    ],
    colors: {
      'editor.background': '#0c0c0e',
      'editor.foreground': '#f4f4f5',
      'editorLineNumber.foreground': '#3f3f46',
      'editorLineNumber.activeForeground': '#a1a1aa',
      'editor.lineHighlightBackground': '#141418',
      'editorGutter.background': '#0c0c0e',
      'editor.selectionBackground': '#1e3a8a',
      'editorWidget.background': '#121216',
      'editorWidget.border': '#27272a',
      'input.background': '#141418',
      'scrollbarSlider.background': '#ffffff14',
      'scrollbarSlider.hoverBackground': '#ffffff24',
      'scrollbarSlider.activeBackground': '#ffffff34',
    },
  })
}

const EXT_LANG_MAP: Record<string, string> = {
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  py: 'python',
  pyw: 'python',
  go: 'go',
  rs: 'rust',
  rb: 'ruby',
  php: 'php',
  java: 'java',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  cc: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  swift: 'swift',
  kt: 'kotlin',
  kts: 'kotlin',
  scala: 'scala',
  dart: 'dart',
  css: 'css',
  scss: 'scss',
  sass: 'scss',
  less: 'less',
  html: 'html',
  htm: 'html',
  xml: 'xml',
  svg: 'xml',
  vue: 'html',
  svelte: 'html',
  json: 'json',
  jsonc: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'ini',
  ini: 'ini',
  env: 'shell',
  md: 'markdown',
  markdown: 'markdown',
  mdx: 'markdown',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  fish: 'shell',
  sql: 'sql',
  graphql: 'graphql',
  gql: 'graphql',
  lua: 'lua',
  r: 'r',
  proto: 'proto',
}

export function detectLanguage(path: string): string {
  const base = path.toLowerCase().split('/').pop() || ''
  if (base === 'dockerfile' || base.startsWith('dockerfile.')) return 'dockerfile'
  if (base === 'makefile' || base === 'gnumakefile') return 'makefile'
  if (base === '.gitignore' || base === '.npmignore' || base === '.dockerignore') return 'shell'
  const ext = base.includes('.') ? base.split('.').pop()! : ''
  return EXT_LANG_MAP[ext] || 'plaintext'
}

export function Mono({ path, value, readOnly, onChange }: {
  path: string
  value: string
  readOnly: boolean
  onChange?: (v: string) => void
}) {
  const onMount: OnMount = (_editor, m) => ensureTheme(m)
  return (
    <Editor
      key={path}
      theme="cursor-dark"
      language={detectLanguage(path)}
      path={path}
      value={value}
      onChange={(v) => onChange?.(v ?? '')}
      onMount={onMount}
      loading={<div className="empty-mid" style={{ position: 'static', paddingTop: 60 }}><div className="spin" /></div>}
      options={{
        readOnly,
        fontFamily: "'Geist Mono', 'JetBrains Mono', 'SF Mono', Menlo, monospace",
        fontSize: 12.5,
        lineHeight: 20,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        wordWrap: 'on',
        tabSize: 2,
        automaticLayout: true,
        padding: { top: 12, bottom: 12 },
        renderLineHighlight: readOnly ? 'none' : 'line',
        occurrencesHighlight: 'off',
        selectionHighlight: false,
        matchBrackets: 'never',
        overviewRulerLanes: 0,
        hideCursorInOverviewRuler: true,
        overviewRulerBorder: false,
        scrollbar: { verticalScrollbarSize: 9, horizontalScrollbarSize: 9, useShadows: false },
        guides: { indentation: false, bracketPairs: false },
        contextmenu: false,
        folding: false,
        glyphMargin: false,
        lineDecorationsWidth: 8,
        lineNumbersMinChars: 3,
        renderWhitespace: 'none',
        bracketPairColorization: { enabled: false },
      }}
    />
  )
}
