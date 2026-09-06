import { useState } from 'react'
import { FolderGit2, Sparkles, FolderSearch } from 'lucide-react'
import { useStore } from '../store'

// Native folder picker, only available inside the Tauri desktop shell (a browser
// can't hand back a real filesystem path). undefined ⇒ not in desktop.
const tauriDialog = (): { open: (o: unknown) => Promise<string | null> } | undefined =>
  (window as unknown as { __TAURI__?: { dialog?: { open: (o: unknown) => Promise<string | null> } } }).__TAURI__?.dialog

// Import screen: point myAudit at a local repo and start an audit run.
export function ConfigScreen() {
  const s = useStore()
  const [repo, setRepo] = useState('')
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const dlg = tauriDialog()

  const browse = async () => {
    if (!dlg) return
    try {
      const picked = await dlg.open({ directory: true, multiple: false, title: 'Select a repository to audit' })
      if (typeof picked === 'string') setRepo(picked)
    } catch (e) { s.toast('error', 'Folder picker failed', (e as Error).message) }
  }

  const start = async () => {
    if (!repo.trim()) { s.toast('error', 'Repo path required'); return }
    setBusy(true)
    await s.createRun({ repo_path: repo.trim(), project: name.trim() || undefined })
    setBusy(false)
  }

  return (
    <div className="wizard-layout">
      <div className="wiz-main wide">
        <div className="wiz-header">
          <h1>Import a codebase</h1>
          <p>Point myAudit at a local repository. It copies the code into an isolated workspace, maps it into modules, runs a QA pass per module to find bugs (with reproduce steps), then autonomously fixes each one and verifies it — all tracked on the board.</p>
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
          <div className="cfg-item">
            <div className="cfg-ico"><Sparkles size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Name (optional)</div><div className="cfg-desc">Defaults to the folder name</div></div>
            <div className="cfg-ctrl">
              <input className="form-input" style={{ width: 340 }} value={name} onChange={(e) => setName(e.target.value)} placeholder="my-project" />
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
          <button className="btn-sm" style={{ padding: '10px 20px' }} disabled={busy} onClick={() => s.setNewOpen(false)}>Cancel</button>
          <button className="btn-sm primary" style={{ padding: '10px 20px' }} disabled={busy} onClick={start}>{busy ? 'Starting…' : 'Start audit →'}</button>
        </div>
      </div>
    </div>
  )
}
