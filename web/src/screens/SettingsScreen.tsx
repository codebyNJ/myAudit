import { useEffect, useState } from 'react'
import { useAppStore } from '../store/slices'
import { api } from '../api'

type Provider = 'claude' | 'opencode'

const defaultOpenCodeModel = 'opencode/big-pickle'
const legacyOpenCodeModel = 'anthropic/claude-haiku-4-5'

const opencodeModels = [
  { id: 'opencode/big-pickle', label: 'Big Pickle — free default' },
  { id: 'opencode/nemotron-3.5-lightning-free', label: 'Nemotron 3.5 Lightning — free' },
  { id: 'opencode/nemotron-3-ultra-free', label: 'Nemotron 3 Ultra — free' },
  { id: 'opencode/mimo-v2.5-free', label: 'MiMo v2.5 — free' },
  { id: 'opencode/ling-3.0-flash-fin-free', label: 'Ling 3.0 Flash — free' },
  { id: 'opencode/muse-spark-1.3-contributor-free', label: 'Muse Spark 1.3 — free' },
  { id: 'opencode/muse-spark-1.2-contributor-free', label: 'Muse Spark 1.2 — free' },
] as const

function normalizeOpenCodeModel(m: string | undefined) {
  if (!m || m === legacyOpenCodeModel) return defaultOpenCodeModel
  return m
}

export function SettingsScreen() {
  const toast = useAppStore((s) => s.toast)
  const [provider, setProvider] = useState<Provider>('claude')
  const [tier, setTier] = useState('haiku-4.5')
  const [opencodeModel, setOpencodeModel] = useState(defaultOpenCodeModel)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getSettings().then((st) => {
      const p = (st.agent_provider as string) || 'claude'
      if (p === 'claude' || p === 'opencode') setProvider(p)
      setTier((st.model_tier as string) || 'haiku-4.5')
      setOpencodeModel(normalizeOpenCodeModel(st.opencode_model as string))
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
      toast('success', 'Settings saved')
    } catch (e) {
      toast('error', 'Save failed', (e as Error).message)
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
            <label style={{ display: 'block', fontSize: 13, marginBottom: 6 }}>OpenCode model</label>
            <select className="set-select" style={{ width: '100%' }} value={opencodeModel} onChange={(e) => setOpencodeModel(e.target.value)}>
              {opencodeModels.map((m) => (
                <option key={m.id} value={m.id}>{m.label}</option>
              ))}
              {!opencodeModels.some((m) => m.id === opencodeModel) && (
                <option value={opencodeModel}>{opencodeModel}</option>
              )}
            </select>
            <p style={{ marginTop: 8, fontSize: 12, color: 'var(--text-muted)' }}>
              Free OpenCode models only — Anthropic models via OpenCode need separate API billing.
              Set <code>OPENCODE_MODEL</code> to use another model (e.g. a paid provider).
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
