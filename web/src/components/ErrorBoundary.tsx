import { Component, type ReactNode } from 'react'

// Top-level guard: screens render agent-produced, variable-shaped data, so a
// single bad render shouldn't white-screen the whole app. Shows a recoverable
// message with a reload instead.
export class ErrorBoundary extends Component<{ children: ReactNode }, { err: Error | null }> {
  state = { err: null as Error | null }
  static getDerivedStateFromError(err: Error) { return { err } }
  componentDidCatch(err: Error) { console.error('myAudit render error:', err) }
  render() {
    if (this.state.err) {
      return (
        <div className="empty-mid">
          <h3>Something went wrong rendering this view</h3>
          <p style={{ maxWidth: 420, color: 'var(--text-muted)' }}>{this.state.err.message}</p>
          <button className="btn-sm primary" style={{ marginTop: 12 }} onClick={() => location.reload()}>Reload</button>
        </div>
      )
    }
    return this.props.children
  }
}
