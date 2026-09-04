import { marked } from 'marked'

marked.setOptions({ breaks: true, gfm: true })

// Renders markdown text (agent messages, summaries) as styled HTML, reusing the
// .md-preview styles. Padding/border are zeroed so it sits inline in chat.
export function Markdown({ text }: { text: string }) {
  return (
    <div
      className="md-preview md-inline"
      style={{ padding: 0, border: 0, overflow: 'visible' }}
      dangerouslySetInnerHTML={{ __html: marked.parse(text || '') as string }}
    />
  )
}
