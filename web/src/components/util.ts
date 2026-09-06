export const STATUS_COLOR: Record<string, string> = {
  done: 'var(--method-get)', complete: 'var(--method-get)', verified: 'var(--method-get)', closed: 'var(--method-get)',
  running: 'var(--method-post)', ready: 'var(--method-post)', in_progress: 'var(--method-post)',
  failed: 'var(--method-del)', reopened: 'var(--method-del)',
  in_review: '#e0a92e', blocked: '#e0a92e', pending: 'var(--text-muted)',
  open: 'var(--text-secondary)', paused: '#e0a92e', dismissed: 'var(--text-dim)', cancelled: 'var(--text-dim)',
}
export const evColor = (k: string) =>
  /fail|red|error/.test(k) ? 'var(--diff-del-text)'
    : /end/.test(k) ? 'var(--diff-add-text)'
    : /checkpoint/.test(k) ? '#e0a92e'
    : /steer/.test(k) ? '#d2a8ff'
    : 'var(--method-post)'
export const fmtTime = (ts: string) => new Date(ts).toLocaleTimeString()
