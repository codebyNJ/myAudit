import { beforeEach, describe, expect, it, vi } from 'vitest'
import { parseHash, useAppStore } from './slices'
import { api } from '../api'

describe('chat tab routing', () => {
  it('is a real tab, so a chat can be linked to and reloaded into', () => {
    expect(parseHash('#/run/abc-123/chat')).toEqual({ runId: 'abc-123', tab: 'chat' })
  })
})

describe('sendChat', () => {
  beforeEach(() => {
    useAppStore.setState({ runId: 'run-1', chatDraft: '', chatBusy: false })
    vi.restoreAllMocks()
  })

  it('clears the draft and reloads the transcript on success', async () => {
    const chat = vi.spyOn(api, 'chat').mockResolvedValue(undefined as never)
    const reload = vi.fn()
    useAppStore.setState({ chatDraft: '  why is this a bug?  ', reloadDetail: reload })

    await useAppStore.getState().sendChat()

    expect(chat).toHaveBeenCalledWith('run-1', 'why is this a bug?')
    expect(useAppStore.getState().chatDraft).toBe('')
    expect(useAppStore.getState().chatBusy).toBe(false)
    expect(reload).toHaveBeenCalled()
  })

  // A ten-minute reply that fails must not also lose what the developer typed.
  it('puts the message back when the send fails', async () => {
    vi.spyOn(api, 'chat').mockRejectedValue(new Error('network down'))
    const toast = vi.fn()
    useAppStore.setState({ chatDraft: 'expensive question', toast, reloadDetail: vi.fn() })

    await useAppStore.getState().sendChat()

    expect(useAppStore.getState().chatDraft).toBe('expensive question')
    expect(useAppStore.getState().chatBusy).toBe(false)
    expect(toast).toHaveBeenCalledWith('error', 'Chat failed', 'network down')
  })

  it('ignores an empty draft and a send already in flight', async () => {
    const chat = vi.spyOn(api, 'chat').mockResolvedValue(undefined as never)

    useAppStore.setState({ chatDraft: '   ' })
    await useAppStore.getState().sendChat()

    useAppStore.setState({ chatDraft: 'hello', chatBusy: true })
    await useAppStore.getState().sendChat()

    expect(chat).not.toHaveBeenCalled()
  })
})
