import { describe, expect, it } from 'vitest'
import { branchCommand, commitCommand, shortSHA } from './gitref'

const RUN = 'fcc9ddf7-379c-40e9-9f12-710e6918a2dd'
const BRANCH = 'myaudit/fcc9ddf7/9b0fe6e7'

describe('branchCommand', () => {
  it('points at the sandbox before the ticket is pushed', () => {
    // The branch exists only in the run's workspace at this point.
    expect(branchCommand(BRANCH, RUN, false)).toBe(`git -C runs/${RUN} show ${BRANCH}`)
  })

  it('points at the developer clone once pushed', () => {
    expect(branchCommand(BRANCH, RUN, true)).toBe(`git fetch origin && git checkout ${BRANCH}`)
  })

  it('never hands over a checkout for a branch that is not in the clone yet', () => {
    expect(branchCommand(BRANCH, RUN, false)).not.toContain('checkout')
  })
})

describe('commitCommand', () => {
  it('addresses the sandbox before push', () => {
    expect(commitCommand('a3f9c21', RUN, false)).toBe(`git -C runs/${RUN} show a3f9c21`)
  })

  it('addresses the clone after push', () => {
    expect(commitCommand('a3f9c21', RUN, true)).toBe('git show a3f9c21')
  })
})

describe('shortSHA', () => {
  it('trims to seven characters', () => {
    expect(shortSHA('a3f9c21deadbeef')).toBe('a3f9c21')
  })

  it('leaves an already-short value alone', () => {
    expect(shortSHA('a3f9')).toBe('a3f9')
  })
})
