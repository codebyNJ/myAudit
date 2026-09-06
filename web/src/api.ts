
export type Run = { id: string; project: string; status: string; created_at: string }
export type Node = { id: string; run_id: string; type: string; status: string; deps: string[] }
export type EventRow = { ts: string; kind: string; level: string; msg: string; node_id?: string }
export type Checkpoint = { id: string; run_id: string; node_id: string; question: string; resolved: boolean; answer: string }
export type FileEntry = { path: string; content?: string; action?: string; changed?: boolean; review?: string }
export type RunDetail = { run: Run; nodes: Node[]; events: EventRow[]; checkpoints: Checkpoint[]; files: FileEntry[]; cost_usd: number }
export type NodeCard = { id: string; type: string; name: string; status: string; deps: number; attempts: number; summary: string; files: number; cost_usd: number; events: number; created_at: string; claimed_at?: string; title?: string; file?: string; severity?: string; priority?: string; detail?: string; tags?: string[] }
export type CreateRunBody = { repo_path: string; project?: string; audit_only?: boolean; budget_usd?: number }
export type SearchHit = { path: string; line: number; text: string }
export type FlowStep = { label: string; file?: string; kind?: string }
export type DataFlow = { name: string; entity?: string; store?: string; steps?: FlowStep[]; note?: string; concern?: string }
export type ProductFlow = { name: string; trigger?: string; steps?: FlowStep[]; outcome?: string; concern?: string }
export type FlowsDoc = { persistence?: string; data_flows?: DataFlow[]; product_flows?: ProductFlow[] }
export type FlowsResp = { ready: boolean; pending: boolean; flows?: FlowsDoc }

async function req<T>(path: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(path, opts)
  if (!r.ok) throw new Error((await r.text()).trim() || `HTTP ${r.status}`)
  return r.status === 204 ? (undefined as T) : ((await r.json()) as T)
}

export const api = {
  health: () => req<{ ready: boolean; claude: boolean; claudeVersion: string; git: boolean; message: string }>('/api/health'),
  listRuns: () => req<Run[]>('/api/runs'),
  runDetail: (id: string) => req<RunDetail>('/api/runs/' + id),
  createRun: (body: CreateRunBody) =>
    req<{ id: string }>('/api/runs', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }),
  resolveCheckpoint: (id: string, answer: string) =>
    req<void>('/api/checkpoints/' + id + '/resolve', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ answer }) }),
  board: (id: string) => req<NodeCard[]>('/api/runs/' + id + '/board'),
  cancelRun: (id: string) => req<void>('/api/runs/' + id + '/cancel', { method: 'POST' }),
  fileContent: (id: string, path: string) => req<{ path: string; content: string }>('/api/runs/' + id + '/file?path=' + encodeURIComponent(path)),
  diff: (id: string, path: string) => req<{ path: string; diff: string }>('/api/runs/' + id + '/diff?path=' + encodeURIComponent(path)),
  saveFile: (id: string, path: string, content: string) =>
    req<void>('/api/runs/' + id + '/file', { method: 'PUT', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ path, content }) }),
  getNotes: (id: string) => req<{ content: string }>('/api/runs/' + id + '/notes'),
  putNotes: (id: string, content: string) =>
    req<void>('/api/runs/' + id + '/notes', { method: 'PUT', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ content }) }),
  chat: (id: string, message: string) =>
    req<{ reply: string }>('/api/runs/' + id + '/chat', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ message }) }),
  setNodeTags: (id: string, nodeId: string, tags: string[]) =>
    req<void>('/api/runs/' + id + '/nodes/' + nodeId + '/tags', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ tags }) }),
  enqueue: (id: string, nodeId: string) =>
    req<void>('/api/runs/' + id + '/nodes/' + nodeId + '/enqueue', { method: 'POST' }),
  patchNode: (id: string, nodeId: string, patch: { severity?: string; priority?: string; status?: string }) =>
    req<void>('/api/runs/' + id + '/nodes/' + nodeId, { method: 'PATCH', headers: { 'content-type': 'application/json' }, body: JSON.stringify(patch) }),
  rawUrl: (id: string, path: string) => '/api/runs/' + id + '/raw?path=' + encodeURIComponent(path),
  reportUrl: (id: string) => '/api/runs/' + id + '/report.md',
  findingsUrl: (id: string) => '/api/runs/' + id + '/findings.json',
  patchUrl: (id: string) => '/api/runs/' + id + '/patch.diff',
  demoPath: () => req<{ path: string }>('/api/demo'),
  review: (id: string, path: string, status: 'accepted' | 'rejected') =>
    req<void>('/api/runs/' + id + '/review', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ path, status }) }),
  search: (id: string, q: string) => req<SearchHit[]>('/api/runs/' + id + '/search?q=' + encodeURIComponent(q)),
  newFile: (id: string, path: string, dir = false) =>
    req<void>('/api/runs/' + id + '/file/new', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ path, dir }) }),
  renameFile: (id: string, from: string, to: string) =>
    req<void>('/api/runs/' + id + '/file/rename', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ from, to }) }),
  deleteFile: (id: string, path: string) =>
    req<void>('/api/runs/' + id + '/file?path=' + encodeURIComponent(path), { method: 'DELETE' }),
  flows: (id: string) => req<FlowsResp>('/api/runs/' + id + '/flows'),
  runFlows: (id: string) => req<void>('/api/runs/' + id + '/flows', { method: 'POST' }),
  getSettings: () => req<Record<string, unknown>>('/api/settings'),
  putSettings: (patch: Record<string, unknown>) =>
    req<Record<string, unknown>>('/api/settings', { method: 'PUT', headers: { 'content-type': 'application/json' }, body: JSON.stringify(patch) }),
}
