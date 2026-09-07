import { marked } from 'marked'

marked.setOptions({ breaks: true, gfm: true })

/** Soft-normalize agent prose so plain newlines and fences render cleanly. */
export function normalizeMarkdown(raw: string): string {
  let t = (raw || '').replace(/\r\n/g, '\n').trim()
  if (!t) return ''
  // Promote common labels into headings so they don't look like a wall of text.
  t = t.replace(
    /^(reproduce|steps to reproduce|expected|actual|why it matters|impact|severity|fix|root cause|recommendation|evidence|location)\s*:?\s*$/gim,
    '### $1',
  )
  // "Label: rest of line" → bold label
  t = t.replace(
    /^(severity|impact|file|line|module|endpoint|component|status)\s*:\s+(.+)$/gim,
    '**$1:** $2',
  )
  // Turn lone numbered lines into a proper list when they appear as "1. text"
  // (already GFM — ensure blank line before lists so marked parses them)
  t = t.replace(/([^\n])\n((?:\d+\.|[-*+])\s)/g, '$1\n\n$2')
  return t
}

export function Markdown({
  text,
  block,
  className,
}: {
  text: string
  block?: boolean
  className?: string
}) {
  const html = marked.parse(normalizeMarkdown(text)) as string
  const cls = [
    'md-preview',
    block ? '' : 'md-inline',
    className || '',
  ].filter(Boolean).join(' ')
  return (
    <div
      className={cls}
      style={block ? { padding: 0, border: 0, overflow: 'visible', display: 'block' } : { padding: 0, border: 0, overflow: 'visible' }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
