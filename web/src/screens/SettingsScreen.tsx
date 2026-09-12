import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { api } from '../api'

type Provider = 'claude' | 'opencode'

export function SettingsScreen() {
  const s = useStore()
  const [provider, setProvider] = useState<Provider>('claude')
  const [tier, setTier] = useState('haiku-4.5')
  const [opencodeModel, setOpencodeModel] = useState('anthropic/claude-haiku-4-5')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getSettings().then((st) => {
      const p = (st.agent_provider as string) || 'claude'
      if (p === 'claude' || p === 'opencode') setProvider(p)
      setTier((st.model_tier as string) || 'haiku-4.5')
      setOpencodeModel((st.opencode_model as string) || 'anthropic/claude-haiku-4-5')
    }).catch(() => {})
  }, [])

  const save = async () => {
    setSaving(true)
    try {
      await api.putSettings({
        agent_provider: provider,
        model_tier: tier,
        opencode_model: opencodeModel,
      })
      s.toast('success', 'Settings saved')
    } catch (e) {
      s.toast('error', 'Save failed', (e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="pane">
      <h2>Settings</h2>
      <p className="sub">Which agent and model the audit uses.</p>

      <div className="set-card">
        <div className="set-h">Agent provider</div>
        <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>CLI harness</label>
        <select className="set-select" style={{ width: '100%' }} value={provider} onChange={(e) => setProvider(e.target.value as Provider)}>
          <option value="claude">Claude Code — <code>claude</code> CLI</option>
          <option value="opencode">OpenCode — <code>opencode</code> CLI</option>
        </select>
        <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
          {provider === 'claude'
            ? 'Authenticate with `claude login`. Set `AGENT_PROVIDER=claude` to override.'
            : 'Authenticate with `opencode auth login`. Set `AGENT_PROVIDER=opencode` to override.'}
        </p>
      </div>

      <div className="set-card">
        <div className="set-h">Model</div>
        {provider === 'claude' ? (
          <>
            <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>Model for QA + fixes</label>
            <select className="set-select" style={{ width: '100%' }} value={tier} onChange={(e) => setTier(e.target.value)}>
              <option value="haiku-4.5">Haiku 4.5 — fast &amp; cheap</option>
              <option value="sonnet-5">Sonnet 5 — balanced (recommended)</option>
              <option value="opus-4.8">Opus 4.8 — deepest, slowest</option>
            </select>
            <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
              A <code>CLAUDE_MODEL</code> environment variable, if set, overrides this.
            </p>
          </>
        ) : (
          <>
            <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>OpenCode model (<code>provider/model</code>)</label>
            <input
              className="set-select"
              style={{ width: '100%', boxSizing: 'border-box' }}
              value={opencodeModel}
              onChange={(e) => setOpencodeModel(e.target.value)}
              placeholder="anthropic/claude-haiku-4-5"
            />
            <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
              A <code>OPENCODE_MODEL</code> environment variable, if set, overrides this.
            </p>
          </>
        )}
      </div>

      <div className="set-card">
        <div className="set-h">Runtime</div>
        <p style={{ fontSize: 12.5, color: 'var(--text-secondary)', lineHeight: 1.6, margin: 0 }}>
          myAudit runs entirely on your machine. The agent uses your logged-in CLI
          and <b>git</b> from your <code>PATH</code> — no API keys stored in myAudit.
          Set <code>AGENT_ISOLATE=1</code> to run Claude agents inside a container (OpenCode runs in the workspace).
        </p>
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn-sm primary" style={{ padding: '8px 16px' }} disabled={saving} onClick={save}>{saving ? 'Saving…' : 'Save settings'}</button>
      </div>
    </div>
  )
}
