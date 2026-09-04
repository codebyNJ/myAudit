import test from 'node:test'
import assert from 'node:assert/strict'
import { runAgent } from '../src/agent.js'
import { createSessionStore } from '../src/session.js'

test('agent loop runs a tool then returns final text', async () => {
  let calls = 0
  const generate = async () => {
    calls++
    if (calls === 1) return { toolCall: { name: 'weather', args: { city: 'SF' } } }
    return { text: 'It is sunny in SF' }
  }
  const tools = { weather: async ({ city }) => ({ temp: 72, city }) }
  const { text, messages } = await runAgent({
    generate,
    tools,
    messages: [{ role: 'user', content: 'weather in SF?' }],
  })
  assert.equal(text, 'It is sunny in SF')
  assert.equal(calls, 2)
  assert.ok(messages.some((m) => m.role === 'tool'), 'tool result appended to convo')
})

test('agent throws on unknown tool', async () => {
  const generate = async () => ({ toolCall: { name: 'ghost', args: {} } })
  await assert.rejects(() => runAgent({ generate, tools: {}, messages: [] }))
})

test('session store appends and reads history', async () => {
  const s = createSessionStore()
  await s.append('c1', { role: 'user', content: 'hi' })
  await s.append('c1', { role: 'assistant', content: 'hello' })
  const h = await s.history('c1')
  assert.equal(h.length, 2)
  await s.reset('c1')
  assert.equal((await s.history('c1')).length, 0)
})
