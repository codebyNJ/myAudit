import test from 'node:test'
import assert from 'node:assert/strict'
import { greet, total } from '../index.js'

test('greet', () => {
  assert.equal(greet('a'), 'hello a')
})

test('total sums all prices', () => {
  assert.equal(total([1, 2, 3]), 6)
})
