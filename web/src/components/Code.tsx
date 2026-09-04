import { useMemo } from 'react'
import hljs from 'highlight.js/lib/common'

// The common bundle registers ~35 languages (go, python, rust, css, yaml,
// markdown, json, typescript, sql, …) — enough to render most imported repos.
const EXT: Record<string, string> = {
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  ts: 'typescript', tsx: 'typescript',
  py: 'python', go: 'go', rs: 'rust', rb: 'ruby', php: 'php', java: 'java',
  c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
  cs: 'csharp', swift: 'swift', kt: 'kotlin', scala: 'scala', dart: 'dart',
  css: 'css', scss: 'scss', less: 'less',
  html: 'xml', htm: 'xml', xml: 'xml', svg: 'xml', vue: 'xml',
  json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'ini', ini: 'ini',
  md: 'markdown', markdown: 'markdown',
  sh: 'bash', bash: 'bash', zsh: 'bash',
  sql: 'sql', graphql: 'graphql', gql: 'graphql', lua: 'lua', r: 'r', pl: 'perl',
}

// langFor returns a registered hljs language id for the path, or null to let
// hljs auto-detect (covers extensions not in the map).
function langFor(path: string): string | null {
  const base = (path.toLowerCase().split('/').pop() || '')
  if (base === 'dockerfile' || base.startsWith('dockerfile.')) return null // not in common → auto
  if (base === 'makefile') return 'makefile'
  const ext = base.includes('.') ? base.split('.').pop()! : ''
  const lang = EXT[ext]
  return lang && hljs.getLanguage(lang) ? lang : null
}

// Syntax-highlighted code, rendered with a line-number gutter.
export function Code({ path, content }: { path: string; content: string }) {
  const lines = useMemo(() => {
    let html: string
    try {
      const lang = langFor(path)
      // highlightAuto over huge files is slow; fall back to escaped plaintext.
      if (!lang && content.length > 200_000) {
        html = escapeHtml(content)
      } else {
        html = lang
          ? hljs.highlight(content, { language: lang, ignoreIllegals: true }).value
          : hljs.highlightAuto(content).value
      }
    } catch {
      html = escapeHtml(content)
    }
    return splitHighlightedLines(html)
  }, [path, content])

  return (
    <div className="diff">
      {lines.map((h, i) => (
        <div className="dl" key={i}>
          <span className="gut">{i + 1}</span>
          <span className="sign"></span>
          <span className="code hljs" dangerouslySetInnerHTML={{ __html: h || ' ' }} />
        </div>
      ))}
    </div>
  )
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c] as string))
}

// Split hljs HTML into per-line HTML, re-opening any spans that cross newlines so
// each line's markup is self-contained.
function splitHighlightedLines(html: string): string[] {
  const out: string[] = []
  const open: string[] = []
  let cur = ''
  const re = /(<span [^>]*>)|(<\/span>)|(\n)|([^<\n]+)|(<[^>]+>)/g
  let m: RegExpExecArray | null
  while ((m = re.exec(html))) {
    if (m[1]) { cur += m[1]; open.push(m[1]) }
    else if (m[2]) { cur += m[2]; open.pop() }
    else if (m[3]) { out.push(cur + '</span>'.repeat(open.length)); cur = open.join('') }
    else { cur += m[0] }
  }
  out.push(cur)
  return out
}
