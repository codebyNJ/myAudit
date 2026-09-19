import { ArrowLeft } from 'lucide-react'
import { AgentAvatar, IcBranch } from './icons'
import { WorkspaceSwitcher } from './WorkspaceSwitcher'
import { Tabs } from './Tabs'
import { AccountMenu } from './AccountMenu'
import { useAppStore } from '../store/slices'
import { api } from '../api'

const RUN_VERB: Record<string, string> = {
  import: 'Importing', map: 'Mapping modules', qa: 'Reviewing', bug: 'Fixing',
}

export function Header() {
  const detail = useAppStore((s) => s.detail)
  const runId = useAppStore((s) => s.runId)
  const toast = useAppStore((s) => s.toast)
  const goHome = useAppStore((s) => s.goHome)
  const reloadDetail = useAppStore((s) => s.reloadDetail)
  const cost = detail?.cost_usd || 0
  const nodes = detail?.nodes || []
  const running = nodes.find((n) => n.status === 'running')
  const queued = nodes.filter((n) => n.status === 'ready' || n.status === 'pending').length
  const runStatus = detail?.run?.status
  const active = runStatus !== 'cancelled' && (!!running || queued > 0)
  const stop = async () => {
    if (!runId) return
    try { await api.cancelRun(runId); toast('info', 'Audit stopped'); reloadDetail() }
    catch (e) { toast('error', 'Stop failed', (e as Error).message) }
  }
  return (
    <header>
      <div className="h-left">
        <div className="logo" style={{ cursor: 'pointer' }} title="Back to dashboard" onClick={goHome}><AgentAvatar size={22} radius={6} /></div>
        <button className="back-btn" title="Back to dashboard" onClick={goHome}><ArrowLeft size={13} /> Dashboard</button>
        <WorkspaceSwitcher />
        {detail?.git?.has_git ? (
          <div className="branch-tag" title={detail.git.remote_url || 'git repo'}>
            <IcBranch /> {detail.git.default_branch || 'main'}
          </div>
        ) : (
          <div className="branch-tag" title="No git remote detected at import"><IcBranch /> no git</div>
        )}
        {cost > 0 && <div className="branch-tag" title="Total model cost for this run">${cost.toFixed(2)}</div>}
        {runStatus === 'cancelled' ? <div className="run-pill idle" title="Audit cancelled">■ cancelled</div>
          : running ? <div className="run-pill" title="The audit is working"><span className="run-dot" />{RUN_VERB[running.type] || running.type}{queued > 0 ? ` · ${queued} queued` : ''}</div>
          : queued > 0 ? <div className="run-pill idle" title="Queued work"><span className="run-dot" />{queued} queued</div>
          : runStatus === 'failed' ? <div className="run-pill failed" title="The audit failed">✗ failed</div>
          : runStatus === 'done' ? <div className="run-pill done" title="Audit complete">✓ done</div>
          : null}
        {active && <button className="btn-sm stop-btn" onClick={stop} title="Stop this audit">■ Stop</button>}
      </div>
      <div className="h-center"><Tabs /></div>
      <div className="h-right"><AccountMenu /></div>
    </header>
  )
}
