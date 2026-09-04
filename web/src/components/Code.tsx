import { useMemo } from 'react'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import xml from 'highlight.js/lib/languages/xml'
import bash from 'highlight.js/lib/languages/bash'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import plaintext from 'highlight.js/lib/languages/plaintext'

hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('dockerfile', dockerfile)
hljs.registerLanguage('plaintext', plaintext)

function langFor(path: string): string {
  const p = path.toLowerCase()
  if (p.endsWith('.js') || p.endsWith('.jsx') || p.endsWith('.mjs') || p.endsWith('.ts') || p.endsWith('.tsx')) return 'javascript'
  if (p.endsWith('.json')) return 'json'
  if (p.endsWith('.html') || p.endsWith('.svg') || p.endsWith('.xml')) return 'xml'
  if (p.endsWith('.sh')) return 'bash'
  if (p.endsWith('dockerfile') || p === 'dockerfile') return 'dockerfile'
  return 'plaintext'
}

// Syntax-highlighted code, rendered with a gutter, in the sample.html diff style.
export function Code({ path, content }: { path: string; content: string }) {
  const lines = useMemo(() => {
    const lang = langFor(path)
    let html: string
    try {
      html = hljs.highlight(content, { language: lang, ignoreIllegals: true }).value
    } catch {
      html = content.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c] as string))
    }
    // hljs may emit spans that cross newlines; split while keeping tags balanced per line
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

// Split hljs HTML into per-line HTML, re-opening any spans that span newlines so
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
