import { describe, expect, it } from 'vitest'
import { shouldRetryPreview } from './store'

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
