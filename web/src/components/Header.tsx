import { AgentAvatar, IcBranch } from './icons'
import { WorkspaceSwitcher } from './WorkspaceSwitcher'
import { Tabs } from './Tabs'
import { AccountMenu } from './AccountMenu'
import { useStore } from '../store'
import { api } from '../api'

// Human labels for the live run-pill (was printing raw node types like "qa running").
const RUN_VERB: Record<string, string> = {
  import: 'Importing', map: 'Mapping modules', qa: 'Reviewing', bug: 'Fixing',
}

// Header: product logo + workspace switcher + branch | centered tabs | account avatar.
// (Search moved into the Explorer.)
export function Header() {
  const s = useStore()
  const cost = s.detail?.cost_usd || 0
  const nodes = s.detail?.nodes || []
  const running = nodes.find((n) => n.status === 'running')
  const queued = nodes.filter((n) => n.status === 'ready' || n.status === 'pending').length
  const active = !!running || queued > 0
  const runStatus = s.detail?.run?.status
  const stop = async () => {
    if (!s.runId) return
    try { await api.cancelRun(s.runId); s.toast('info', 'Audit stopped'); s.reloadDetail() }
    catch (e) { s.toast('error', 'Stop failed', (e as Error).message) }
  }
  return (
    <header>
      <div className="h-left">
        <div className="logo" style={{ cursor: 'pointer' }} title="Back to projects" onClick={s.goHome}><AgentAvatar size={22} radius={6} /></div>
        <WorkspaceSwitcher />
        <div className="branch-tag"><IcBranch /> main</div>
        {cost > 0 && <div className="branch-tag" title="Total model cost for this run">${cost.toFixed(2)}</div>}
        {running
          ? <div className="run-pill" title="The audit is working"><span className="run-dot" />{RUN_VERB[running.type] || running.type}{queued > 0 ? ` · ${queued} queued` : ''}</div>
          : queued > 0 ? <div className="run-pill idle" title="Queued work"><span className="run-dot" />{queued} queued</div>
          : runStatus === 'failed' ? <div className="run-pill failed" title="The audit failed">✗ failed</div>
          : runStatus === 'cancelled' ? <div className="run-pill idle" title="Audit cancelled">■ cancelled</div>
          : runStatus === 'done' ? <div className="run-pill done" title="Audit complete">✓ done</div>
          : null}
        {active && <button className="btn-sm stop-btn" onClick={stop} title="Stop this audit">■ Stop</button>}
      </div>
      <div className="h-center"><Tabs /></div>
      <div className="h-right"><AccountMenu /></div>
    </header>
  )
}
