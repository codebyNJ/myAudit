import { describe, expect, it } from 'vitest'
import { parseHash, shouldRetryPreview } from './store'

describe('shouldRetryPreview', () => {
  it('fires on the first check for a fresh run', () => {
    expect(shouldRetryPreview(-1, 0)).toBe(true)
  })

  it('fires again when another node completes', () => {
    expect(shouldRetryPreview(1, 2)).toBe(true)
  })

  it('does not fire when the done count is unchanged', () => {
    expect(shouldRetryPreview(2, 2)).toBe(false)
  })
})

describe('parseHash', () => {
  it('reads the run id and tab', () => {
    expect(parseHash('#/run/abc-123/dev')).toEqual({ runId: 'abc-123', tab: 'dev' })
  })

  it('defaults to the board when no tab is given', () => {
    expect(parseHash('#/run/abc-123')).toEqual({ runId: 'abc-123', tab: 'kanban' })
  })

  it('rewrites the legacy activity tab to playwright', () => {
    // 'activity' was this screen's earlier name and still appears in saved links.
    expect(parseHash('#/run/abc-123/activity').tab).toBe('playwright')
  })

  it('falls back to the board for an unknown tab', () => {
    expect(parseHash('#/run/abc-123/nonsense').tab).toBe('kanban')
  })

  it('returns no run for the dashboard hash', () => {
    expect(parseHash('#/')).toEqual({ runId: null, tab: 'kanban' })
    expect(parseHash('')).toEqual({ runId: null, tab: 'kanban' })
  })
})
