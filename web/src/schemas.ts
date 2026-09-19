import { z } from 'zod'

// Schemas written against the **Go structs**, not against the TypeScript types
// they replace. The types had drifted: LiveView declared `At`/`Title` while the
// server sends `at`/`title` (fixed in #46 — both reads were permanently
// undefined), and NodeCard was missing `category` and `confidence`, which the
// server computes, stores and ships on every board card.
//
// Two rules that fall out of how Go marshals, worth knowing before editing:
//
//   1. Nothing here is ever `null`. Every pointer field carries `omitempty`,
//      and Go omits a nil pointer rather than emitting null. So `.optional()`,
//      never `.nullable()` — adding nullable would hide the contract rather
//      than describe it. `settings` is the one exception: it is a bare
//      map[string]any and can serialise as null.
//
//   2. Slices that the handler nil-guards are always present. `.default([])`
//      is belt-and-braces against a server regression, and lets the hand-rolled
//      `normalizeRunDetail` and ~30 `|| []` guards go away.

// ---------------------------------------------------------------- primitives

const uuid = z.string()
const timestamp = z.string()

// ------------------------------------------------------------------ entities

// internal/store/read.go:12-17 (RunSummary)
export const RunSchema = z.object({
  id: uuid,
  project: z.string(),
  status: z.string(),
  created_at: timestamp,
})

// internal/store/runs.go:10-16 — `deps` has no omitempty and scanIDs never
// returns nil, so it is always an array.
export const NodeSchema = z.object({
  id: uuid,
  run_id: uuid,
  type: z.string(),
  status: z.string(),
  deps: z.array(uuid).default([]),
})

// internal/store/read.go:19-25 — node_id is *uuid.UUID + omitempty: absent, never null.
export const EventRowSchema = z.object({
  ts: timestamp,
  kind: z.string(),
  level: z.string(),
  msg: z.string(),
  node_id: uuid.optional(),
})

// internal/store/checkpoints.go:12-19 — all six always present; `answer` is
// COALESCE'd to '' in SQL.
export const CheckpointSchema = z.object({
  id: uuid,
  run_id: uuid,
  node_id: uuid,
  question: z.string(),
  resolved: z.boolean(),
  answer: z.string(),
})

// internal/store/read.go:148-154. `changed` has no omitempty → always present.
// `content` and `action` are declared on the Go struct but `listWorkspaceFiles`
// (internal/api/views.go:31) only ever sets Path/Changed/Review, so they never
// appear in the /api/runs/{id} payload. Kept optional rather than deleted,
// because ChangedFilesForRun does populate Action for the export paths.
export const FileEntrySchema = z.object({
  path: z.string(),
  changed: z.boolean().default(false),
  review: z.string().optional(),
  content: z.string().optional(),
  action: z.string().optional(),
})

// internal/store/opts.go:10-15
export const GitInfoSchema = z.object({
  has_git: z.boolean(),
  remote_url: z.string().optional(),
  default_branch: z.string().optional(),
  head_sha: z.string().optional(),
})

// internal/api/api.go:22-30 — all four slices nil-guarded by the handler
// (api.go:69-85); git is *store.GitInfo + omitempty.
export const RunDetailSchema = z.object({
  run: RunSchema,
  nodes: z.array(NodeSchema).default([]),
  events: z.array(EventRowSchema).default([]),
  checkpoints: z.array(CheckpointSchema).default([]),
  files: z.array(FileEntrySchema).default([]),
  cost_usd: z.number(),
  git: GitInfoSchema.optional(),
})

// internal/store/read.go:56-82 (NodeDetail).
// `category` and `confidence` were absent from the old TS type despite being
// selected by NodeDetailsForRun and shipped on every card — #25 needs them.
// `tags` has no omitempty and scanTags never returns nil, so it is required,
// which retires ten dead `(n.tags || [])` guards.
export const NodeCardSchema = z.object({
  id: uuid,
  type: z.string(),
  name: z.string(),
  status: z.string(),
  deps: z.number(),
  attempts: z.number(),
  summary: z.string(),
  files: z.number(),
  cost_usd: z.number(),
  events: z.number(),
  created_at: timestamp,
  claimed_at: timestamp.optional(),

  title: z.string().optional(),
  file: z.string().optional(),
  severity: z.string().optional(),
  priority: z.string().optional(),
  class: z.string().optional(),
  category: z.string().optional(),
  confidence: z.string().optional(),
  detail: z.string().optional(),
  tags: z.array(z.string()).default([]),

  commit_sha: z.string().optional(),
  pr_url: z.string().optional(),
  pr_status: z.string().optional(),
})

// internal/api/files.go:128-132 (searchHit)
export const SearchHitSchema = z.object({
  path: z.string(),
  line: z.number(),
  text: z.string(),
})

// ---------------------------------------------------------------------- flows
// internal/worker/flows.go:17-44

export const FlowStepSchema = z.object({
  label: z.string(),
  file: z.string().optional(),
  kind: z.string().optional(),
})

export const DataFlowSchema = z.object({
  name: z.string(),
  entity: z.string().optional(),
  store: z.string().optional(),
  steps: z.array(FlowStepSchema).default([]),
  note: z.string().optional(),
  concern: z.string().optional(),
})

export const ProductFlowSchema = z.object({
  name: z.string(),
  trigger: z.string().optional(),
  steps: z.array(FlowStepSchema).default([]),
  outcome: z.string().optional(),
  concern: z.string().optional(),
})

export const FlowsDocSchema = z.object({
  persistence: z.string().optional(),
  data_flows: z.array(DataFlowSchema).default([]),
  product_flows: z.array(ProductFlowSchema).default([]),
})

// The /flows envelope is hand-built with fmt.Fprintf (internal/api/api.go:370-377)
// and splices in a json.RawMessage read straight from nodes.output — agent-authored
// JSON that nothing validates server-side. This is the single highest-value
// schema in the file.
export const FlowsRespSchema = z.object({
  ready: z.boolean(),
  pending: z.boolean(),
  flows: FlowsDocSchema.optional(),
})

// ----------------------------------------------------------------- live/preview

// internal/api/live.go:21-30. `status` is one of the four writeJSON exits;
// `kind` comes from preview.Detect and is always one of three values.
export const LiveViewSchema = z.object({
  status: z.enum(['live', 'frames', 'idle', 'crashed']),
  kind: z.enum(['web', 'desktop', 'none']).optional(),
  url: z.string().optional(),
  frame: z.string().optional(),
  at: z.string().optional(),
  title: z.string().optional(),
  reason: z.string().optional(),
  latency_ms: z.number().optional(),
})

// internal/api/live.go:103-117 — three exits, and one of them is a bare 202
// with no body at all, which is the normal "starting up" path. Hence the
// .optional() at the root: this endpoint legitimately returns nothing.
export const StartPreviewSchema = z
  .object({
    status: z.string().optional(),
    url: z.string().optional(),
    kind: z.string().optional(),
  })
  .optional()

// ------------------------------------------------------------- misc responses

export const RunIdSchema = z.object({ id: uuid })
export const FileContentSchema = z.object({ path: z.string(), content: z.string() })
export const DiffSchema = z.object({ path: z.string(), diff: z.string() })
export const NotesSchema = z.object({ content: z.string() })
export const ChatReplySchema = z.object({ reply: z.string() })
export const DemoPathSchema = z.object({ path: z.string() })
export const PushPRSchema = z.object({ pr_url: z.string(), branch: z.string() })

// internal/api/health.go:44-58 — an untyped map literal, but every key is
// always written.
export const HealthSchema = z.object({
  ready: z.boolean(),
  git: z.boolean(),
  gh: z.object({ installed: z.boolean(), authenticated: z.boolean() }).optional(),
  agentProvider: z.string(),
  providers: z.record(
    z.string(),
    z.object({ installed: z.boolean(), version: z.string().optional() }),
  ),
  claude: z.boolean(),
  claudeVersion: z.string(),
  message: z.string(),
})

// internal/store/notes.go:43-52 — genuinely untyped map[string]any, and the
// only place in this API that can serialise as null (a nil map from a `null`
// settings.data column). Passthrough because rejecting unknown keys would
// break legacy rows; nullable+transform so App.tsx and SettingsScreen can stop
// casting an unchecked value.
export const SettingsSchema = z
  .looseObject({
    agent_provider: z.enum(['claude', 'opencode']).optional(),
    model_tier: z.string().optional(),
    opencode_model: z.string().optional(),
  })
  .nullable()
  .transform((v) => v ?? {})

// ------------------------------------------------------------ request bodies

export const CreateRunBodySchema = z.object({
  repo_path: z.string().min(1),
  project: z.string().optional(),
  audit_only: z.boolean().optional(),
  budget_usd: z.number().optional(),
})

export const ResolveCheckpointBodySchema = z.object({ answer: z.string() })
export const SaveFileBodySchema = z.object({ path: z.string().min(1), content: z.string() })
export const PutNotesBodySchema = z.object({ content: z.string() })
export const ChatBodySchema = z.object({ message: z.string().min(1) })
export const SetTagsBodySchema = z.object({ tags: z.array(z.string()) })
export const PatchNodeBodySchema = z.object({
  severity: z.enum(['high', 'medium', 'low']).optional(),
  priority: z.enum(['P0', 'P1', 'P2']).optional(),
  status: z.enum(['open', 'dismissed', 'in_review', 'done']).optional(),
})
export const ReviewBodySchema = z.object({
  path: z.string().min(1),
  status: z.enum(['accepted', 'rejected']),
})
export const NewFileBodySchema = z.object({ path: z.string().min(1), dir: z.boolean() })
export const RenameFileBodySchema = z.object({ from: z.string().min(1), to: z.string().min(1) })

// ---------------------------------------------------------------------- types
// Inferred from the schemas so they can no longer drift from the wire format.

export type Run = z.infer<typeof RunSchema>
export type Node = z.infer<typeof NodeSchema>
export type EventRow = z.infer<typeof EventRowSchema>
export type Checkpoint = z.infer<typeof CheckpointSchema>
export type FileEntry = z.infer<typeof FileEntrySchema>
export type GitInfo = z.infer<typeof GitInfoSchema>
export type RunDetail = z.infer<typeof RunDetailSchema>
export type NodeCard = z.infer<typeof NodeCardSchema>
export type SearchHit = z.infer<typeof SearchHitSchema>
export type FlowStep = z.infer<typeof FlowStepSchema>
export type DataFlow = z.infer<typeof DataFlowSchema>
export type ProductFlow = z.infer<typeof ProductFlowSchema>
export type FlowsDoc = z.infer<typeof FlowsDocSchema>
export type FlowsResp = z.infer<typeof FlowsRespSchema>
export type LiveView = z.infer<typeof LiveViewSchema>
export type Health = z.infer<typeof HealthSchema>
export type Settings = z.infer<typeof SettingsSchema>
export type CreateRunBody = z.infer<typeof CreateRunBodySchema>
export type PatchNodeBody = z.infer<typeof PatchNodeBodySchema>
