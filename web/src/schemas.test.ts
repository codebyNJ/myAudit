import { describe, expect, it, vi, afterEach } from 'vitest'
import {
  FlowsRespSchema, LiveViewSchema, NodeCardSchema, RunDetailSchema, SettingsSchema,
  StartPreviewSchema,
} from './schemas'
import { parsed } from './validate'

// These tests encode the wire contract with the Go server. Each one exists
// because the TypeScript type disagreed with the Go struct and nobody noticed.

describe('LiveViewSchema', () => {
  it('reads the lowercase keys the server actually sends', () => {
    // internal/api/live.go:26-27 marshals `at` and `title`. The old TS type
    // declared `At`/`Title`, so AgentScreen read undefined forever (#46).
    const wire = { status: 'frames', kind: 'none', frame: '.myaudit/live/s1.png', at: '2026-09-19T11:28:35Z' }
    const v = LiveViewSchema.parse(wire)
    expect(v.at).toBe('2026-09-19T11:28:35Z')
  })

  it('rejects a status outside the four the server can write', () => {
    expect(LiveViewSchema.safeParse({ status: 'starting' }).success).toBe(false)
  })

  it('accepts the idle payload, which omits everything optional', () => {
    expect(LiveViewSchema.parse({ status: 'idle', kind: 'none' }).url).toBeUndefined()
  })
})

describe('NodeCardSchema', () => {
  const base = {
    id: 'n1', type: 'bug', name: 'n', status: 'open', deps: 0, attempts: 1,
    summary: '', files: 0, cost_usd: 0, events: 0, created_at: '2026-01-01T00:00:00Z',
  }

  it('keeps category and confidence, which the old type dropped', () => {
    // read.go:74-75 ships both on every card; the TS type omitted them, so the
    // UI could not see data the server was already sending. #25 needs these.
    const c = NodeCardSchema.parse({ ...base, category: 'security', confidence: 'high' })
    expect(c.category).toBe('security')
    expect(c.confidence).toBe('high')
  })

  it('always yields an array for tags', () => {
    // Tags has no omitempty and scanTags never returns nil, so `tags` is always
    // present — the ten `(n.tags || [])` guards were dead weight.
    expect(NodeCardSchema.parse(base).tags).toEqual([])
    expect(NodeCardSchema.parse({ ...base, tags: ['a'] }).tags).toEqual(['a'])
  })
})

describe('RunDetailSchema', () => {
  it('defaults the four slices so callers can iterate unguarded', () => {
    const d = RunDetailSchema.parse({
      run: { id: 'r1', project: 'p', status: 'running', created_at: '2026-01-01T00:00:00Z' },
      cost_usd: 0,
    })
    expect(d.nodes).toEqual([])
    expect(d.events).toEqual([])
    expect(d.checkpoints).toEqual([])
    expect(d.files).toEqual([])
  })

  it('treats git as absent rather than null', () => {
    // Go omits a nil pointer; it never emits null. A null here means the
    // server changed, and we want to hear about it.
    const ok = RunDetailSchema.safeParse({
      run: { id: 'r1', project: 'p', status: 'x', created_at: '2026-01-01T00:00:00Z' },
      cost_usd: 0, git: null,
    })
    expect(ok.success).toBe(false)
  })
})

describe('StartPreviewSchema', () => {
  it('accepts no body at all, which is the normal start path', () => {
    // internal/api/live.go:117 returns a bare 202 while the server boots.
    expect(StartPreviewSchema.parse(undefined)).toBeUndefined()
  })

  it('accepts the unsupported payload including its kind key', () => {
    expect(StartPreviewSchema.parse({ status: 'unsupported', kind: 'none' })?.kind).toBe('none')
  })
})

describe('SettingsSchema', () => {
  it('turns a null settings blob into an empty object', () => {
    // The only endpoint that can serialise as null: a nil map[string]any.
    // App.tsx and SettingsScreen read keys off this without checking.
    expect(SettingsSchema.parse(null)).toEqual({})
  })

  it('keeps unknown keys so legacy rows survive', () => {
    const s = SettingsSchema.parse({ agent_provider: 'claude', theme: 'dark', token_budget: 100000 })
    expect(s.agent_provider).toBe('claude')
    expect((s as Record<string, unknown>).theme).toBe('dark')
  })
})

describe('FlowsRespSchema', () => {
  it('accepts the not-ready envelope, which omits flows', () => {
    expect(FlowsRespSchema.parse({ ready: false, pending: true }).flows).toBeUndefined()
  })

  it('validates the agent-authored blob the server passes through unchecked', () => {
    // internal/api/api.go:377 splices nodes.output->$.flows into the response
    // with fmt.Fprintf and never validates it. This is the only check it gets.
    const bad = FlowsRespSchema.safeParse({ ready: true, pending: false, flows: { data_flows: [{ entity: 'x' }] } })
    expect(bad.success).toBe(false) // a flow with no name
  })
})

describe('parsed()', () => {
  afterEach(() => vi.unstubAllEnvs())

  it('throws in dev so drift is impossible to miss', () => {
    vi.stubEnv('DEV', true)
    expect(() => parsed(LiveViewSchema, { status: 'nope' }, 'live', { status: 'idle' as const }))
      .toThrow(/failed schema validation/)
  })

  it('warns and falls back in prod so the UI keeps rendering', () => {
    vi.stubEnv('DEV', false)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const fallback = { status: 'idle' as const }
    expect(parsed(LiveViewSchema, { status: 'nope' }, 'live', fallback)).toBe(fallback)
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })

  it('names the failing field so the log is actionable', () => {
    vi.stubEnv('DEV', true)
    expect(() => parsed(LiveViewSchema, { status: 'nope' }, 'live', { status: 'idle' as const }))
      .toThrow(/status/)
  })
})
