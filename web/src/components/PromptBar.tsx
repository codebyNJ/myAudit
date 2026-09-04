import { useState } from 'react'
import { IcFile, IcPlus, IcChevron, IcSend } from './icons'

// The IDE-style composer. Context chips are real: "Add Context" opens a picker
// of the run's files; chosen files are sent along with the steer message.
export function PromptBar({
  files, meta, disabled, autoFocus, onSend,
}: {
  files: string[]
  meta?: string
  disabled?: boolean
  autoFocus?: boolean
  onSend: (text: string, context: string[]) => void
}) {
  const [context, setContext] = useState<string[]>([])
  const [picker, setPicker] = useState(false)
  const short = (p: string) => p.split('/').pop()
  const add = (p: string) => { setContext((c) => c.includes(p) ? c : [...c, p]); setPicker(false) }
  const remove = (p: string) => setContext((c) => c.filter((x) => x !== p))

  return (
    <div className="composer">
      <form className="prompt-box" onSubmit={(e) => {
        e.preventDefault()
        const t = (e.currentTarget.elements.namedItem('m') as HTMLTextAreaElement)
        if (t.value.trim()) { onSend(t.value.trim(), context); t.value = ''; setContext([]) }
      }}>
        <div className="prompt-context">
          {context.map((p) => (
            <div key={p} className="context-pill" onClick={() => remove(p)} title="Remove">
              <span className="icon"><IcFile /></span>{short(p)} <span style={{ color: 'var(--text-muted)' }}>×</span>
            </div>
          ))}
          <div style={{ position: 'relative' }}>
            <div className="context-pill" style={{ borderStyle: 'dashed', background: 'transparent' }} onClick={() => setPicker((v) => !v)}>
              <span className="icon"><IcPlus /></span> Add Context
            </div>
            {picker && (
              <div className="dropdown" style={{ bottom: 30, left: 0, minWidth: 220, maxHeight: 220, overflow: 'auto' }}>
                {files.length ? files.filter((f) => !context.includes(f)).map((f) => (
                  <div key={f} className="dd-item" onClick={() => add(f)}><IcFile /> <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{f}</span></div>
                )) : <div style={{ padding: 10, color: 'var(--text-muted)', fontSize: 12 }}>No files yet</div>}
              </div>
            )}
          </div>
        </div>
        <textarea name="m" className="prompt-input" rows={1} disabled={disabled} autoFocus={autoFocus}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              e.currentTarget.form?.requestSubmit()
            }
          }}
          placeholder={disabled ? 'Create a project in Config to start…' : 'Ask the intern to edit code, or type / for commands...'} />
        <div className="prompt-footer">
          <div className="pf-left">
            <button type="button" className="model-select">opus-4.8 <IcChevron size={10} /></button>
            {meta && <span className="token-count">{meta}</span>}
          </div>
          <div className="pf-right"><button type="submit" className="btn-send" disabled={disabled}><IcSend /></button></div>
        </div>
      </form>
    </div>
  )
}
