import test from 'node:test'
import assert from 'node:assert/strict'
import { buildOpenApi } from '../src/openapi.js'

test('builds an OpenAPI 3 spec from routes', () => {
  const spec = buildOpenApi({
    title: 'acme',
    version: '1',
    routes: [
      { method: 'POST', path: '/v1/auth/login', summary: 'login' },
      { method: 'GET', path: '/v1/me', summary: 'current user' },
    ],
  })
  assert.equal(spec.openapi, '3.0.0')
  assert.equal(spec.info.title, 'acme')
  assert.equal(spec.paths['/v1/auth/login'].post.summary, 'login')
  assert.ok(spec.paths['/v1/me'].get.responses['200'])
})

test('merges methods under the same path', () => {
  const spec = buildOpenApi({
    routes: [
      { method: 'GET', path: '/v1/item', summary: 'list' },
      { method: 'POST', path: '/v1/item', summary: 'create' },
    ],
  })
  assert.ok(spec.paths['/v1/item'].get)
  assert.ok(spec.paths['/v1/item'].post)
})
