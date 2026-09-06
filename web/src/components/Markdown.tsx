import { marked } from 'marked'

marked.setOptions({ breaks: true, gfm: true })

export function Markdown({ text }: { text: string }) {
  return (
    <div
      className="md-preview md-inline"
      style={{ padding: 0, border: 0, overflow: 'visible' }}
      dangerouslySetInnerHTML={{ __html: marked.parse(text || '') as string }}
    />
  )
}
