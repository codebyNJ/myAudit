import { useEffect, useRef, useState } from 'react'
import { useStore } from '../store'

type Pt = { x: number; y: number }

// A free, draggable ER canvas: entity nodes can be moved anywhere by dragging
// their header. Positions seed from a grid the first time a node appears.
export function SchemaScreen() {
  const s = useStore()
  const schema = s.schema || []
  const [pos, setPos] = useState<Record<string, Pt>>({})
  const drag = useRef<{ name: string; sx: number; sy: number; ox: number; oy: number } | null>(null)

  const names = schema.map((e) => e.name).join(',')
  useEffect(() => {
    setPos((p) => {
      const next = { ...p }
      schema.forEach((e, i) => {
        if (!next[e.name]) next[e.name] = { x: 60 + (i % 3) * 300, y: 50 + Math.floor(i / 3) * 250 }
      })
      return next
    })
  }, [names]) // eslint-disable-line react-hooks/exhaustive-deps

  const onDown = (name: string) => (e: React.MouseEvent) => {
    e.preventDefault()
    const cur = pos[name] || { x: 80, y: 80 }
    drag.current = { name, sx: e.clientX, sy: e.clientY, ox: cur.x, oy: cur.y }
    const move = (ev: MouseEvent) => {
      const d = drag.current
      if (!d) return
      setPos((p) => ({ ...p, [d.name]: { x: d.ox + (ev.clientX - d.sx), y: d.oy + (ev.clientY - d.sy) } }))
    }
    const up = () => { drag.current = null; window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up) }
    window.addEventListener('mousemove', move)
    window.addEventListener('mouseup', up)
  }

  if (s.schema == null) return <div className="empty-mid"><div className="spin" /></div>
  if (!schema.length) return <div className="empty-mid"><h3>No schema</h3></div>
  return (
    <div className="schema-board">
      {schema.map((e) => {
        const p = pos[e.name] || { x: 80, y: 80 }
        return (
          <div className="schema-node" key={e.name} style={{ left: p.x, top: p.y }}>
            <div className="sn-head" style={{ cursor: 'grab', userSelect: 'none' }} onMouseDown={onDown(e.name)}>
              {e.name} <span>{e.name}</span>
            </div>
            <div className="sn-body">
              {e.fields.map((f) => (
                <div className="sn-field" key={f.name}>
                  <span className={f.ref ? 'fk' : f.name === '_id' ? 'pk' : ''}>{f.name}</span>
                  <span className="type">{f.ref ? f.ref : f.type || ''}</span>
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
