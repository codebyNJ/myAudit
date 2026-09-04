import { useState } from 'react'
import {
  Server, Users, Database, Box, Plus, Trash2, Check, ShieldCheck, Container, Sparkles,
} from 'lucide-react'
import { useStore } from '../store'
import type { Resource } from '../api'

const FIELD_TYPES = ['string', 'number', 'boolean', 'date']

export function ConfigScreen() {
  const s = useStore()
  const [name, setName] = useState('acme-saas')
  const [tenancy, setTenancy] = useState('multi')
  const [cache, setCache] = useState('shadow')
  const [resources, setResources] = useState<Resource[]>([
    { name: 'Project', fields: [{ name: 'title', type: 'string' }, { name: 'status', type: 'string' }] },
  ])
  const [busy, setBusy] = useState(false)

  const patch = (fn: (r: Resource[]) => void) => setResources((rs) => { const c = structuredClone(rs); fn(c); return c })
  const addResource = () => patch((r) => r.push({ name: '', fields: [{ name: '', type: 'string' }] }))
  const removeResource = (i: number) => patch((r) => r.splice(i, 1))
  const addField = (i: number) => patch((r) => r[i].fields.push({ name: '', type: 'string' }))
  const removeField = (i: number, j: number) => patch((r) => r[i].fields.splice(j, 1))

  const scaffold = async () => {
    const clean = resources
      .map((r) => ({ name: r.name.trim(), fields: r.fields.filter((f) => f.name.trim()).map((f) => ({ name: f.name.trim(), type: f.type })) }))
      .filter((r) => r.name)
    setBusy(true)
    // createRun lands on Dev in a locked, prompt-only state; scaffolding runs in the background.
    await s.createRun({ project: name.trim() || 'acme-saas', tenancy, cache, resources: clean })
    setBusy(false)
  }

  return (
    <div className="wizard-layout">
      <div className="wiz-main wide">
        <div className="wiz-header">
          <h1>New Project</h1>
          <p>The template ships the whole SaaS base (auth, workspaces, multi-tenancy, RBAC, cache, frontend) for $0. Describe the domain resources you want — the intern generates a feature module for each, following the template's own conventions.</p>
        </div>

        {/* Project */}
        <div className="cfg-section">
          <div className="cfg-section-h">Project</div>
          <div className="cfg-item">
            <div className="cfg-ico"><Server size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Service name</div><div className="cfg-desc">Rewrites the template identity everywhere</div></div>
            <div className="cfg-ctrl"><input className="form-input" style={{ width: 220 }} value={name} onChange={(e) => setName(e.target.value)} placeholder="core-api" /></div>
          </div>
          <div className="cfg-item">
            <div className="cfg-ico"><Users size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Tenancy</div><div className="cfg-desc">Workspace isolation model</div></div>
            <div className="cfg-ctrl">
              <div className="seg-pill">
                <button className={tenancy === 'multi' ? 'on' : ''} onClick={() => setTenancy('multi')}>Multi-tenant</button>
                <button className={tenancy === 'single' ? 'on' : ''} onClick={() => setTenancy('single')}>Single</button>
              </div>
            </div>
          </div>
          <div className="cfg-item">
            <div className="cfg-ico"><Database size={16} /></div>
            <div className="cfg-text"><div className="cfg-title">Valkey cache</div><div className="cfg-desc">Membership/authz cache mode (fail-closed to Mongo)</div></div>
            <div className="cfg-ctrl">
              <div className="seg-pill">
                {['off', 'shadow', 'on'].map((m) => <button key={m} className={cache === m ? 'on' : ''} onClick={() => setCache(m)}>{m}</button>)}
              </div>
            </div>
          </div>
        </div>

        {/* Domain resources */}
        <div className="cfg-section">
          <div className="cfg-section-h" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>Domain resources</span>
            <button className="btn-sm" onClick={addResource}><Plus size={12} /> Add resource</button>
          </div>
          <div style={{ padding: '4px 14px 14px', display: 'flex', flexDirection: 'column', gap: 12 }}>
            {resources.length === 0 && <div className="tree-empty" style={{ padding: 16 }}>No resources — the app will scaffold with only the built-in Item example. Add one to generate a feature.</div>}
            {resources.map((r, i) => (
              <div key={i} className="res-card" style={{ border: '1px solid var(--border)', borderRadius: 8, padding: 12 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                  <Box size={14} />
                  <input className="form-input" style={{ flex: 1, fontWeight: 600 }} value={r.name} onChange={(e) => patch((rs) => { rs[i].name = e.target.value })} placeholder="Resource name (e.g. Project)" />
                  <button className="btn-sm" title="Remove resource" onClick={() => removeResource(i)}><Trash2 size={12} /></button>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {r.fields.map((f, j) => (
                    <div key={j} style={{ display: 'flex', gap: 6, alignItems: 'center', paddingLeft: 22 }}>
                      <input className="form-input" style={{ flex: 1 }} value={f.name} onChange={(e) => patch((rs) => { rs[i].fields[j].name = e.target.value })} placeholder="field name" />
                      <select className="form-input" style={{ width: 120 }} value={f.type} onChange={(e) => patch((rs) => { rs[i].fields[j].type = e.target.value })}>
                        {FIELD_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                      </select>
                      <button className="btn-sm" title="Remove field" onClick={() => removeField(i, j)}><Trash2 size={11} /></button>
                    </div>
                  ))}
                  <button className="btn-sm ghost" style={{ marginLeft: 22, alignSelf: 'flex-start' }} onClick={() => addField(i)}><Plus size={11} /> field</button>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Included (always-on, free from the template) */}
        <div className="cfg-section">
          <div className="cfg-section-h">Included free (from the template)</div>
          <div className="badge-wrap">
            <span className="inc-badge"><ShieldCheck size={12} /> Auth + RBAC + refresh cookie</span>
            <span className="inc-badge"><Users size={12} /> Workspaces + multi-tenancy</span>
            <span className="inc-badge"><Database size={12} /> Membership cache</span>
            <span className="inc-badge"><Container size={12} /> Docker Compose</span>
            <span className="inc-badge"><Check size={12} /> React + RTK frontend</span>
          </div>
          <div className="cfg-section-h" style={{ paddingTop: 4 }}><Sparkles size={12} style={{ marginRight: 2 }} /> The intern generates</div>
          <div className="badge-wrap">
            {resources.filter((r) => r.name.trim()).map((r) => <span key={r.name} className="inc-badge add"><Box size={12} /> {r.name.trim()} module</span>)}
            {resources.filter((r) => r.name.trim()).length === 0 && <span className="cfg-desc">— add a resource above —</span>}
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
          <button className="btn-sm" style={{ padding: '10px 20px' }} disabled={busy}>Cancel</button>
          <button className="btn-sm primary" style={{ padding: '10px 20px' }} disabled={busy} onClick={scaffold}>{busy ? 'Creating…' : 'Generate →'}</button>
        </div>
      </div>
    </div>
  )
}
