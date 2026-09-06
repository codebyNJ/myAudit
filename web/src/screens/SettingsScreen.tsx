import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { api } from '../api'

export function SettingsScreen() {
  const s = useStore()
  const [tier, setTier] = useState('haiku-4.5')
  const [saving, setSaving] = useState(false)
  useEffect(() => { api.getSettings().then((st) => setTier((st.model_tier as string) || 'haiku-4.5')).catch(() => {}) }, [])
  const save = async () => {
    setSaving(true)
    try { await api.putSettings({ model_tier: tier }); s.toast('success', 'Settings saved') }
    catch (e) { s.toast('error', 'Save failed', (e as Error).message) }
    finally { setSaving(false) }
  }
  return (
    <div className="pane">
      <h2>Settings</h2>
      <p className="sub">Which model the audit uses, and what it runs on.</p>

      <div className="set-card">
        <div className="set-h">Model</div>
        <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>Model for QA + fixes</label>
        <select className="set-select" style={{ width: '100%' }} value={tier} onChange={(e) => setTier(e.target.value)}>
          <option value="haiku-4.5">Haiku 4.5 — fast &amp; cheap</option>
          <option value="sonnet-5">Sonnet 5 — balanced (recommended)</option>
          <option value="opus-4.8">Opus 4.8 — deepest, slowest</option>
        </select>
        <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>Applies to the next node the agent runs. A <code>CLAUDE_MODEL</code> environment variable, if set, overrides this.</p>
      </div>

      <div className="set-card">
        <div className="set-h">Runtime</div>
        <p style={{ fontSize: 12.5, color: 'var(--text-secondary)', lineHeight: 1.6, margin: 0 }}>
          myAudit runs entirely on your machine. The agent uses your logged-in <b>Claude Code</b> CLI
          (<code>claude</code>) and <b>git</b> from your <code>PATH</code> — no API keys, no accounts, nothing stored.
          Set <code>AGENT_ISOLATE=1</code> to run each agent inside a container.
        </p>
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn-sm primary" style={{ padding: '8px 16px' }} disabled={saving} onClick={save}>{saving ? 'Saving…' : 'Save settings'}</button>
      </div>
    </div>
  )
}
