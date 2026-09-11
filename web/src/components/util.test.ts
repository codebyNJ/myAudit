import { describe, expect, it } from 'vitest'
import { evColor, fmtTime } from './util'

describe('evColor', () => {
  it('maps failure events to red', () => {
    expect(evColor('node.fail')).toBe('var(--diff-del-text)')
  })

  it('maps end events to green', () => {
    expect(evColor('node.end')).toBe('var(--diff-add-text)')
  })

  it('defaults to post color', () => {
    expect(evColor('node.start')).toBe('var(--method-post)')
  })
})

describe('fmtTime', () => {
  it('formats an ISO timestamp', () => {
    const out = fmtTime('2026-01-15T14:30:00.000Z')
    expect(out).toMatch(/\d/)
  })
})
