import { describe, expect, it } from 'vitest'
import reactPkg from 'react/package.json'
import reactDomPkg from 'react-dom/package.json'

// React 19 refuses to run when react and react-dom are different versions: it
// throws minified error #527 at mount and the entire UI renders blank. A
// dependency bump that moves one and not the other is enough to do it, and
// nothing else in CI notices — lint, typecheck, build and the node-environment
// unit tests all pass against a mismatched pair, because none of them mount the
// app. This landed on main once already (react 19.3.0 / react-dom 19.2.8).
describe('react / react-dom version parity', () => {
  it('resolves both packages to the same version', () => {
    expect(reactDomPkg.version).toBe(reactPkg.version)
  })
})
