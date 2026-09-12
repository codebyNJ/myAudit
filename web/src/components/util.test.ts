import { describe, expect, it } from 'vitest'
import { evColor, fmtTime, viewportWidth, withPath } from './util'

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

describe('viewportWidth', () => {
  it('returns undefined for desktop (full width)', () => {
    expect(viewportWidth('desktop')).toBeUndefined()
  })
  it('returns 768 for tablet', () => {
    expect(viewportWidth('tablet')).toBe(768)
  })
  it('returns 390 for mobile', () => {
    expect(viewportWidth('mobile')).toBe(390)
  })
})

describe('withPath', () => {
  it('returns the base url unchanged for an empty path', () => {
    expect(withPath('http://localhost:41000', '')).toBe('http://localhost:41000')
  })
  it('returns the base url unchanged for the root path', () => {
    expect(withPath('http://localhost:41000', '/')).toBe('http://localhost:41000')
  })
  it('appends a path that is missing its leading slash', () => {
    expect(withPath('http://localhost:41000', 'checkout')).toBe('http://localhost:41000/checkout')
  })
  it('appends a path that already has a leading slash', () => {
    expect(withPath('http://localhost:41000', '/checkout')).toBe('http://localhost:41000/checkout')
  })
  it('strips a trailing slash from the base url before appending', () => {
    expect(withPath('http://localhost:41000/', '/checkout')).toBe('http://localhost:41000/checkout')
  })
})
