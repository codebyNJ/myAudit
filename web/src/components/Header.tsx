import { AgentAvatar, IcBranch } from './icons'
import { WorkspaceSwitcher } from './WorkspaceSwitcher'
import { Tabs } from './Tabs'
import { AccountMenu } from './AccountMenu'
import { useStore } from '../store'

// Header: product logo + workspace switcher + branch | centered tabs | account avatar.
// (Search moved into the Explorer.)
export function Header() {
  const s = useStore()
  const cost = s.detail?.cost_usd || 0
  const nodes = s.detail?.nodes || []
  const running = nodes.find((n) => n.status === 'running')
  const queued = nodes.filter((n) => n.status === 'ready' || n.status === 'pending').length
  return (
    <header>
      <div className="h-left">
        <div className="logo" style={{ cursor: 'pointer' }} title="Back to projects" onClick={s.goHome}><AgentAvatar size={22} radius={6} /></div>
        <WorkspaceSwitcher />
        <div className="branch-tag"><IcBranch /> main</div>
        {cost > 0 && <div className="branch-tag" title="Total model cost for this run">${cost.toFixed(2)}</div>}
        {running
          ? <div className="run-pill" title="The audit is working"><span className="run-dot" />{running.type} running{queued > 0 ? ` · ${queued} queued` : ''}</div>
          : queued > 0 && <div className="run-pill idle" title="Queued work"><span className="run-dot" />{queued} queued</div>}
      </div>
      <div className="h-center"><Tabs /></div>
      <div className="h-right"><AccountMenu /></div>
    </header>
  )
}
