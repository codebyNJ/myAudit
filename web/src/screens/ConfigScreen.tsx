import { useState } from 'react'
import { FolderGit2, Sparkles, FolderSearch, Shield, Wallet } from 'lucide-react'
import { useStore } from '../store'

const tauriDialog = (): { open: (o: unknown) => Promise<string | null> } | undefined =>
  (window as unknown as { __TAURI__?: { dialog?: { open: (o: unknown) => Promise<string | null> } } }).__TAURI__?.dialog

export function ConfigScreen() {
  const s = useStore()
  const [repo, setRepo] = useState('')
  const [name, setName] = useState('')
  const [auditOnly, setAuditOnly] = useState(false)
  const [budget, setBudget] = useState('')
  const [busy, setBusy] = useState(false)
  const dlg = tauriDialog()

  const browse = async () => {
    if (!dlg) return
    try {
      const picked = await dlg.open({ directory: true, multiple: false, title: 'Select a repository to audit' })
      if (typeof picked === 'string') setRepo(picked)
    } catch (e) { s.toast('error', 'Folder picker failed', (e as Error).message) }
  }

  const path = repo.trim()
  const isAbs = /^(\/|[A-Za-z]:[\\/])/.test(path)
  const pathError = path !== '' && !isAbs ? 'Enter an absolute path (e.g. /Users/you/project)' : ''
  const budgetNum = parseFloat(budget)
  const budgetUSD = budget !== '' && !Number.isNaN(budgetNum) && budgetNum > 0 ? budgetNum : undefined

  const start = async () => {
    if (!path) { s.toast('error', 'Repo path required'); return }
    if (!isAbs) { s.toast('error', 'Path must be absolute', 'e.g. /Users/you/project'); return }
    setBusy(true)
    await s.createRun({
      repo_path: path,
      project: name.trim() || undefined,
      audit_only: auditOnly || undefined,
      budget_usd: budgetUSD,
    })
    setBusy(false)
  }

  return (
    <div className="wizard-layout">
      <div className="wiz-main wide">
        <div className="wiz-header">
          <h1>Import a codebase</h1>
          <p>Point myAudit at a local repository. It copies the code into an isolated workspace, maps modules, runs QA, and (unless you choose audit-only) fixes what it finds.</p>
        </div>

        <div className="cfg-section">
          <div className="cfg-section-h">Repository</div>
          <div className="cfg-item">
            <div className="cfg-ico"><FolderGit2 size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Local path</div><div className="cfg-desc">Absolute path to the repo to audit</div></div>
            <div className="cfg-ctrl" style={{ display: 'flex', gap: 8 }}>
              <input className="form-input" style={{ width: dlg ? 250 : 340 }} value={repo} onChange={(e) => setRepo(e.target.value)}
                placeholder="/Users/you/code/my-project"
                onKeyDown={(e) => { if (e.key === 'Enter') start() }} />
              {dlg && <button className="btn-sm" style={{ flex: 'none' }} onClick={browse} title="Choose a folder"><FolderSearch size={13} /> Browse…</button>}
            </div>
          </div>
          {pathError && <div className="cfg-item"><div className="cfg-ico" /><div style={{ color: 'var(--method-del)', fontSize: 12 }}>{pathError}</div></div>}
          {!dlg && <div className="cfg-item"><div className="cfg-ico" /><div style={{ color: 'var(--text-muted)', fontSize: 12 }}>Tip: folder-browse is available in the desktop app; in the browser, paste an absolute path.</div></div>}
          <div className="cfg-item">
            <div className="cfg-ico"><Sparkles size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Name (optional)</div><div className="cfg-desc">Defaults to the folder name</div></div>
            <div className="cfg-ctrl">
              <input className="form-input" style={{ width: 340 }} value={name} onChange={(e) => setName(e.target.value)} placeholder="my-project" />
            </div>
          </div>
        </div>

        <div className="cfg-section">
          <div className="cfg-section-h">Run options</div>
          <div className="cfg-item">
            <div className="cfg-ico"><Shield size={16} /></div>
            <div className="cfg-text">
              <div className="cfg-title">Audit only</div>
              <div className="cfg-desc">Find issues; don’t auto-fix. Use “Fix this” on a ticket when you’re ready.</div>
            </div>
            <div className="cfg-ctrl">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer' }}>
                <input type="checkbox" checked={auditOnly} onChange={(e) => setAuditOnly(e.target.checked)} />
                Findings only
              </label>
            </div>
          </div>
          <div className="cfg-item">
            <div className="cfg-ico"><Wallet size={16} /></div>
            <div className="cfg-text">
              <div className="cfg-title">Budget cap (USD)</div>
              <div className="cfg-desc">Stop when spend hits this. Small repo on Haiku is often ~$1–3; Sonnet more.</div>
            </div>
            <div className="cfg-ctrl">
              <input className="form-input" style={{ width: 120 }} type="number" min="0" step="0.5" value={budget}
                onChange={(e) => setBudget(e.target.value)} placeholder="e.g. 5" />
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
          <button className="btn-sm" style={{ padding: '10px 20px' }} disabled={busy} onClick={() => s.setNewOpen(false)}>Cancel</button>
          <button className="btn-sm primary" style={{ padding: '10px 20px' }} disabled={busy || !!pathError || !path} onClick={start}>{busy ? 'Starting…' : 'Start audit →'}</button>
        </div>
      </div>
    </div>
  )
}
