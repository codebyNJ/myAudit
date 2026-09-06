// Renders a unified git diff with per-line coloring. Hunk headers, additions,
// and deletions are tinted; file headers are dimmed. Read-only review view.
export function Diff({ text }: { text: string }) {
  if (!text.trim()) return <div className="diff-empty">No changes against the imported baseline.</div>
  const lines = text.split('\n')
  return (
    <pre className="diff">
      {lines.map((ln, i) => {
        let cls = 'dl'
        if (ln.startsWith('+++') || ln.startsWith('---')) cls = 'dl dl-file'
        else if (ln.startsWith('@@')) cls = 'dl dl-hunk'
        else if (ln.startsWith('diff ') || ln.startsWith('index ') || ln.startsWith('new file') || ln.startsWith('deleted file')) cls = 'dl dl-meta'
        else if (ln.startsWith('+')) cls = 'dl dl-add'
        else if (ln.startsWith('-')) cls = 'dl dl-del'
        return <div key={i} className={cls}>{ln || ' '}</div>
      })}
    </pre>
  )
}
