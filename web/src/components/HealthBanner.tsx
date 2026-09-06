import { useEffect, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { api } from '../api'

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
