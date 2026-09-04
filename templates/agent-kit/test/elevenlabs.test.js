import test from 'node:test'
import assert from 'node:assert/strict'
import { tts } from '../src/elevenlabs.js'

test('tts posts to the voice endpoint with the api key header', async () => {
  let captured
  const fetchImpl = async (url, opts) => {
    captured = { url, opts }
    return { ok: true, arrayBuffer: async () => new ArrayBuffer(8) }
  }
  const buf = await tts({ text: 'hello', voiceId: 'v1', apiKey: 'secret', fetchImpl })
  assert.ok(buf.byteLength === 8)
  assert.match(captured.url, /\/v1\/text-to-speech\/v1$/)
  assert.equal(captured.opts.headers['xi-api-key'], 'secret')
  assert.match(captured.opts.body, /hello/)
})

test('tts requires an api key', async () => {
  await assert.rejects(() => tts({ text: 'x', voiceId: 'v', apiKey: '', fetchImpl: async () => ({}) }))
})

test('tts throws on non-ok response', async () => {
  const fetchImpl = async () => ({ ok: false, status: 429 })
  await assert.rejects(() => tts({ text: 'x', voiceId: 'v', apiKey: 'k', fetchImpl }), /429/)
})
