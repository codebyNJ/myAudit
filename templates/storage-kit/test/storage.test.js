import test from 'node:test'
import assert from 'node:assert/strict'
import { createStorage } from '../src/storage.js'

function fakeClient() {
  const sends = []
  return {
    sends,
    async send(d) {
      sends.push(d)
      return d.op === 'GetObject' ? { Body: 'the-bytes' } : {}
    },
  }
}

test('put sends a PutObject with bucket/key/content-type and returns a url', async () => {
  const client = fakeClient()
  const s = createStorage({ client, bucket: 'uploads', publicBase: 'http://localhost:9000' })
  const res = await s.put('avatars/a.png', 'BODY', 'image/png')
  const sent = client.sends[0]
  assert.equal(sent.op, 'PutObject')
  assert.equal(sent.Bucket, 'uploads')
  assert.equal(sent.Key, 'avatars/a.png')
  assert.equal(sent.ContentType, 'image/png')
  assert.equal(res.url, 'http://localhost:9000/uploads/avatars/a.png')
})

test('get returns the object body', async () => {
  const client = fakeClient()
  const s = createStorage({ client, bucket: 'uploads' })
  assert.equal(await s.get('a.png'), 'the-bytes')
})

test('bucket is required', () => {
  assert.throws(() => createStorage({ client: fakeClient() }))
})
