import Editor, { loader, type OnMount } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'

// Bundle Monaco into the app so it runs offline inside the embedded Go binary
// (the default @monaco-editor/react loads from a CDN, which CSP + offline block).
// We deliberately don't ship the language web-workers: they only add background
// TS type-checking / JSON-schema validation, which a read-and-review surface
// doesn't need. Everything the user sees — syntax highlighting, line numbers,
// minimap, find/replace, folding, word-wrap, multi-cursor — runs on the main
// thread and works without them. A silent stub keeps Monaco happy.
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
  m.editor.defineTheme('myaudit', {
    base: 'vs-dark', inherit: true, rules: [],
    colors: {
      'editor.background': '#09090b',
      'editor.foreground': '#fafafa',
      'editorLineNumber.foreground': '#3f3f46',
      'editorLineNumber.activeForeground': '#a1a1aa',
      'editor.lineHighlightBackground': '#141417',
      'editorGutter.background': '#09090b',
      'editor.selectionBackground': '#264f78',
      'editorWidget.background': '#141417',
      'editorWidget.border': '#27272a',
      'input.background': '#141417',
    },
  })
}

// langFor maps a file path to a Monaco language id (Monaco auto-detects many,
// but explicit ids give reliable highlighting).
const EXT: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  py: 'python', go: 'go', rs: 'rust', rb: 'ruby', php: 'php', java: 'java', c: 'c', h: 'c',
  cpp: 'cpp', cc: 'cpp', cs: 'csharp', swift: 'swift', kt: 'kotlin', scala: 'scala', dart: 'dart',
  css: 'css', scss: 'scss', less: 'less', html: 'html', htm: 'html', xml: 'xml', svg: 'xml', vue: 'html',
  json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'ini', ini: 'ini', md: 'markdown', markdown: 'markdown',
  sh: 'shell', bash: 'shell', zsh: 'shell', sql: 'sql', graphql: 'graphql', lua: 'lua', r: 'r',
}
function langFor(path: string): string | undefined {
  const base = (path.toLowerCase().split('/').pop() || '')
  if (base === 'dockerfile') return 'dockerfile'
  if (base === 'makefile') return 'makefile'
  const ext = base.includes('.') ? base.split('.').pop()! : ''
  return EXT[ext]
}

// Read/edit surface. readOnly=true → a proper code viewer (line numbers, find,
// folding, minimap) instead of a plain textarea.
export function Mono({ path, value, readOnly, onChange }: {
  path: string; value: string; readOnly: boolean; onChange?: (v: string) => void
}) {
  const onMount: OnMount = (_editor, m) => ensureTheme(m)
  return (
    <Editor
      key={path}
      theme="myaudit"
      language={langFor(path)}
      path={path}
      value={value}
      onChange={(v) => onChange?.(v ?? '')}
      onMount={onMount}
      loading={<div className="empty-mid" style={{ position: 'static', paddingTop: 60 }}><div className="spin" /></div>}
      options={{
        readOnly,
        fontFamily: "'Geist Mono','JetBrains Mono','SF Mono',monospace",
        fontSize: 12.5,
        lineHeight: 20,
        minimap: { enabled: true },
        scrollBeyondLastLine: false,
        wordWrap: 'off',
        smoothScrolling: true,
        renderWhitespace: 'none',
        tabSize: 2,
        automaticLayout: true,
        padding: { top: 12, bottom: 12 },
      }}
    />
  )
}

