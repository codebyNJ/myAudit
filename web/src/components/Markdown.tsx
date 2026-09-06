import { marked } from 'marked'

marked.setOptions({ breaks: true, gfm: true })

export function Markdown({ text, block }: { text: string; block?: boolean }) {
  return (
    <div
      className={block ? 'md-preview' : 'md-preview md-inline'}
      style={{ padding: 0, border: 0, overflow: 'visible' }}
      dangerouslySetInnerHTML={{ __html: marked.parse(text || '') as string }}
    />
  )
}
