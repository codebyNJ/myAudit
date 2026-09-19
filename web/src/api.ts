import type { ZodType } from 'zod'
import { parsed } from './validate'
import {
  ChatBodySchema, ChatReplySchema, CreateRunBodySchema, DemoPathSchema, DiffSchema,
  FileContentSchema, FlowsRespSchema, HealthSchema, LiveViewSchema, NewFileBodySchema,
  NodeCardSchema, NotesSchema, PatchNodeBodySchema, PushPRSchema, PutNotesBodySchema,
  RenameFileBodySchema, ResolveCheckpointBodySchema, ReviewBodySchema, RunDetailSchema,
  RunIdSchema, RunSchema, SaveFileBodySchema, SearchHitSchema, SetTagsBodySchema,
  SettingsSchema, StartPreviewSchema,
} from './schemas'

export type {
  Checkpoint, CreateRunBody, DataFlow, EventRow, FileEntry, FlowStep, FlowsDoc, FlowsResp,
  GitInfo, Health, LiveView, Node, NodeCard, ProductFlow, Run, RunDetail, SearchHit, Settings,
} from './schemas'
import type { NodeCard, Run, RunDetail, SearchHit } from './schemas'

/** Send a JSON body, validated on the way out so a malformed request is caught here. */
function json<T>(schema: ZodType<T>, body: T): RequestInit {
  return {
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(schema.parse(body)),
  }
}

/**
 * Fetch and decode. Non-2xx throws with the server's plain-text message; an
 * empty body decodes to `undefined`, which is correct for the fourteen
 * endpoints that return 200/201/202 with nothing — and for startPreview, whose
 * normal path is a bare 202 (internal/api/live.go:117).
 */
async function decode(path: string, opts?: RequestInit): Promise<unknown> {
  const r = await fetch(path, opts)
  const body = await r.text()
  if (!r.ok) throw new Error(body.trim() || `HTTP ${r.status}`)
  if (!body.trim()) return undefined
  try {
    return JSON.parse(body)
  } catch {
    throw new Error(`bad JSON from ${path}: ${body.slice(0, 120)}`)
  }
}

/** Fetch, decode, and validate against the wire contract. */
async function req<T>(path: string, schema: ZodType<T>, fallback: T, opts?: RequestInit): Promise<T> {
  return parsed(schema, await decode(path, opts), `GET ${path}`, fallback)
}

/** Fetch an endpoint that returns no body. */
async function send(path: string, opts?: RequestInit): Promise<void> {
  await decode(path, opts)
}

const runPath = (id: string) => '/api/runs/' + id
const nodePath = (id: string, nid: string) => runPath(id) + '/nodes/' + nid

export const api = {
  health: () => req('/api/health', HealthSchema, {
    ready: false, git: false, agentProvider: '', providers: {},
    claude: false, claudeVersion: '', message: '',
  }),

  listRuns: () => req<Run[]>('/api/runs', RunSchema.array(), []),

  runDetail: (id: string) =>
    req<RunDetail>(runPath(id), RunDetailSchema, {
      run: { id, project: '', status: '', created_at: '' },
      nodes: [], events: [], checkpoints: [], files: [], cost_usd: 0,
    }),

  createRun: (body: import('./schemas').CreateRunBody) =>
    req('/api/runs', RunIdSchema, { id: '' }, { method: 'POST', ...json(CreateRunBodySchema, body) }),

  resolveCheckpoint: (id: string, answer: string) =>
    send('/api/checkpoints/' + id + '/resolve', {
      method: 'POST', ...json(ResolveCheckpointBodySchema, { answer }),
    }),

  board: (id: string) => req<NodeCard[]>(runPath(id) + '/board', NodeCardSchema.array(), []),

  cancelRun: (id: string) => send(runPath(id) + '/cancel', { method: 'POST' }),

  fileContent: (id: string, path: string) =>
    req(runPath(id) + '/file?path=' + encodeURIComponent(path), FileContentSchema, { path, content: '' }),

  diff: (id: string, path: string) =>
    req(runPath(id) + '/diff?path=' + encodeURIComponent(path), DiffSchema, { path, diff: '' }),

  saveFile: (id: string, path: string, content: string) =>
    send(runPath(id) + '/file', { method: 'PUT', ...json(SaveFileBodySchema, { path, content }) }),

  getNotes: (id: string) => req(runPath(id) + '/notes', NotesSchema, { content: '' }),

  putNotes: (id: string, content: string) =>
    send(runPath(id) + '/notes', { method: 'PUT', ...json(PutNotesBodySchema, { content }) }),

  chat: (id: string, message: string) =>
    req(runPath(id) + '/chat', ChatReplySchema, { reply: '' }, {
      method: 'POST', ...json(ChatBodySchema, { message }),
    }),

  setNodeTags: (id: string, nodeId: string, tags: string[]) =>
    send(nodePath(id, nodeId) + '/tags', { method: 'POST', ...json(SetTagsBodySchema, { tags }) }),

  enqueue: (id: string, nodeId: string) => send(nodePath(id, nodeId) + '/enqueue', { method: 'POST' }),

  patchNode: (id: string, nodeId: string, patch: import('./schemas').PatchNodeBody) =>
    send(nodePath(id, nodeId), { method: 'PATCH', ...json(PatchNodeBodySchema, patch) }),

  pushPR: (id: string, nodeId: string) =>
    req(nodePath(id, nodeId) + '/push-pr', PushPRSchema, { pr_url: '', branch: '' }, { method: 'POST' }),

  rawUrl: (id: string, path: string) => runPath(id) + '/raw?path=' + encodeURIComponent(path),
  reportUrl: (id: string) => runPath(id) + '/report.md',
  findingsUrl: (id: string) => runPath(id) + '/findings.json',
  patchUrl: (id: string) => runPath(id) + '/patch.diff',

  demoPath: () => req('/api/demo', DemoPathSchema, { path: '' }),

  review: (id: string, path: string, status: 'accepted' | 'rejected') =>
    send(runPath(id) + '/review', { method: 'POST', ...json(ReviewBodySchema, { path, status }) }),

  search: (id: string, q: string) =>
    req<SearchHit[]>(runPath(id) + '/search?q=' + encodeURIComponent(q), SearchHitSchema.array(), []),

  newFile: (id: string, path: string, dir = false) =>
    send(runPath(id) + '/file/new', { method: 'POST', ...json(NewFileBodySchema, { path, dir }) }),

  renameFile: (id: string, from: string, to: string) =>
    send(runPath(id) + '/file/rename', { method: 'POST', ...json(RenameFileBodySchema, { from, to }) }),

  deleteFile: (id: string, path: string) =>
    send(runPath(id) + '/file?path=' + encodeURIComponent(path), { method: 'DELETE' }),

  startPreview: (id: string) =>
    req(runPath(id) + '/preview', StartPreviewSchema, undefined, { method: 'POST' }),

  stopPreview: (id: string) => send(runPath(id) + '/preview', { method: 'DELETE' }),

  live: (id: string) => req(runPath(id) + '/live', LiveViewSchema, { status: 'idle' as const }),

  previewRestart: (id: string) => send(runPath(id) + '/preview/restart', { method: 'POST' }),

  // Deliberately outside the schema boundary: text/plain, not JSON.
  previewLog: async (id: string): Promise<string> => {
    const r = await fetch(runPath(id) + '/preview/log')
    return r.ok ? r.text() : ''
  },

  flows: (id: string) => req(runPath(id) + '/flows', FlowsRespSchema, { ready: false, pending: false }),

  runFlows: (id: string) => send(runPath(id) + '/flows', { method: 'POST' }),

  getSettings: () => req('/api/settings', SettingsSchema, {}),

  putSettings: (patch: Record<string, unknown>) =>
    req('/api/settings', SettingsSchema, {}, {
      method: 'PUT',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(patch),
    }),
}
