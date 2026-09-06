import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { api } from '../api'
import { AgentAvatar } from '../components/icons'

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
  const integ: [string, string][] = [['Claude CLI', 'connected'], ['GitHub', 'connected'], ['ELEVENLABS_API_KEY', 'not set'], ['S3 / MinIO', 'connected']]
  return (
    <div className="pane">
      <h2>Account &amp; Settings</h2>
      <p className="sub">Profile, model policy, and integrations.</p>

      <div className="set-card">
        <div className="set-h">Account</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ width: 40, height: 40, borderRadius: 8, overflow: 'hidden' }}><AgentAvatar size={40} radius={0} /></div>
          <div><div style={{ fontWeight: 600 }}>nijeesh</div><div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Pro · connected</div></div>
        </div>
      </div>

      <div className="set-card">
        <div className="set-h">Model policy</div>
        <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>Model for QA + fixes</label>
        <select className="set-select" style={{ width: '100%' }} value={tier} onChange={(e) => setTier(e.target.value)}>
          <option value="haiku-4.5">Haiku 4.5 — fast &amp; cheap</option>
          <option value="sonnet-5">Sonnet 5 — balanced (recommended)</option>
          <option value="opus-4.8">Opus 4.8 — deepest, slowest</option>
        </select>
        <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>Applies to the next node the agent runs. A <code>CLAUDE_MODEL</code> environment variable, if set, overrides this.</p>
      </div>

      <div className="set-card">
        <div className="set-h">Integrations</div>
        {integ.map(([k, v]) => (
          <div className="set-row" key={k}><span>{k}</span><span style={{ color: v === 'connected' ? 'var(--method-get)' : 'var(--text-muted)' }}>{v}</span></div>
        ))}
        <p style={{ marginTop: 10, fontSize: 12, color: 'var(--text-muted)' }}>Keys are read from the environment — presence shown, never the value.</p>
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn-sm primary" style={{ padding: '8px 16px' }} disabled={saving} onClick={save}>{saving ? 'Saving…' : 'Save settings'}</button>
      </div>
    </div>
  )
}
