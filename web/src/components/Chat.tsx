import { useStore } from '../store'
import { api } from '../api'
import { AgentAvatar, IcCheck } from './icons'
import { PromptBar } from './PromptBar'
import { Markdown } from './Markdown'
import { evColor } from './util'

// The right-docked chat layer from sample.html. Always visible in Dev, even with
// no project (shows the default greeting + composer).
export function Chat() {
  const s = useStore()
  const events = s.detail?.events || []
  const cp = s.detail?.checkpoints?.[0]
  const project = (s.runs || []).find((r) => r.id === s.runId)?.project
  const files = (s.detail?.files || []).map((f) => f.path)

  const send = async (t: string, context: string[]) => {
    if (!s.runId) return
    const msg = context.length ? t + '  (context: ' + context.join(', ') + ')' : t
    try { await api.steer(s.runId, msg); s.toast('success', 'Sent', msg); s.reloadDetail() }
    catch (e) { s.toast('error', 'Steer failed', (e as Error).message) }
  }

  return (
    <div className="chatlayer">
      <div className="bubbles">
        <div className="mb-agent">
          <div className="agent-header"><div className="agent-icon" style={{ background: 'transparent', width: 20, height: 20 }}><AgentAvatar size={20} /></div>myAudit</div>

          {events.length > 0 && (
            <div className="thinking-trace" style={{ flexDirection: 'column', alignItems: 'stretch', gap: 8 }}>
              {events.slice(-5).map((e, i) => (
                <div className="trace-item" key={i} style={{ flexDirection: 'column', alignItems: 'flex-start', gap: 2 }}>
                  <span style={{ color: evColor(e.kind), display: 'flex', gap: 6, alignItems: 'center', fontFamily: 'var(--font-mono)', fontSize: 11 }}><IcCheck /> {e.kind}</span>
                  {e.msg && (e.msg.length > 60 || /[*`#\-]/.test(e.msg)
                    ? <Markdown text={e.msg} />
                    : <span style={{ color: 'var(--text-secondary)' }}>{e.msg}</span>)}
                </div>
              ))}
            </div>
          )}

          {cp
            ? <div className="agent-reply" style={{ border: '1px solid #e0a92e', borderRadius: 12, padding: 12 }}>
                <b>◆ Checkpoint:</b> {cp.question}
                <div style={{ marginTop: 10, display: 'flex', gap: 8 }}>
                  <button className="btn-sm" onClick={() => s.resolveCheckpoint(cp.id, 'skip')}>Skip</button>
                  <button className="btn-sm primary" onClick={() => s.resolveCheckpoint(cp.id, 'confirm')}>Confirm</button>
                </div>
              </div>
            : <div className="agent-reply">
                {project
                  ? <>Auditing <b>{project}</b> — reading the code, mapping flows, generating a test, and reviewing for bugs. Ask me anything about it.</>
                  : <>Hi — I audit codebases. Import a repo from the home screen and I'll interrogate it here.</>}
              </div>}
        </div>
      </div>

      <PromptBar files={files} meta={`${events.length} events`} disabled={!s.runId} autoFocus={false} onSend={send} />
    </div>
  )
}
