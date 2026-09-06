import { useEffect, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { api } from '../api'

// Preflight banner: warns when the Claude CLI or git is missing, so a first-run
// user isn't left with a silently-dead audit. Renders nothing when all is well.
export function HealthBanner() {
  const [msg, setMsg] = useState('')
  useEffect(() => {
    let alive = true
    api.health().then((h) => { if (alive && !h.ready) setMsg(h.message) }).catch(() => {})
    return () => { alive = false }
  }, [])
  if (!msg) return null
  return (
    <div className="health-banner">
      <AlertTriangle size={15} /> <span>{msg}</span>
    </div>
  )
}
