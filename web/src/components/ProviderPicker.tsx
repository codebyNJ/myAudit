import { useState } from 'react'
import { createPortal } from 'react-dom'
import { api } from '../api'

type Provider = 'claude' | 'opencode'

export function ProviderPicker({ onDone }: { onDone: () => void }) {
  const [provider, setProvider] = useState<Provider>('claude')
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try {
      await api.putSettings({ agent_provider: provider })
      onDone()
    } catch {
      setSaving(false)
    }
  }

  return createPortal(
    <div className="as-portal" role="dialog" aria-modal="true" aria-label="Choose agent provider">
      <div className="as-scrim" />
      <div className="provider-picker">
        <h2>Choose your agent</h2>
        <p className="sub">myAudit shells out to a local CLI on your machine. Pick which one to use for QA and fixes.</p>
        <div className="provider-cards">
          <button type="button" className={`provider-card ${provider === 'claude' ? 'on' : ''}`} onClick={() => setProvider('claude')}>
            <div className="provider-name">Claude Code</div>
            <p>Uses the <code>claude</code> CLI with your existing Claude Code login. Run <code>claude login</code> first.</p>
          </button>
          <button type="button" className={`provider-card ${provider === 'opencode' ? 'on' : ''}`} onClick={() => setProvider('opencode')}>
            <div className="provider-name">OpenCode</div>
            <p>Uses the <code>opencode</code> CLI with multi-provider models. Run <code>opencode auth login</code> first.</p>
          </button>
        </div>
        <div className="provider-actions">
          <button className="btn-sm primary" style={{ padding: '8px 20px' }} disabled={saving} onClick={save}>
            {saving ? 'Saving…' : 'Continue'}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
