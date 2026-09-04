export const STATUS_COLOR: Record<string, string> = {
  done: 'var(--method-get)', complete: 'var(--method-get)', running: 'var(--method-post)', ready: 'var(--method-post)',
  failed: 'var(--method-del)', blocked: '#e0a92e', pending: 'var(--text-muted)',
}
export const evColor = (k: string) =>
  /fail|red|error/.test(k) ? 'var(--diff-del-text)'
    : /end/.test(k) ? 'var(--diff-add-text)'
    : /checkpoint/.test(k) ? '#e0a92e'
    : /steer/.test(k) ? '#d2a8ff'
    : 'var(--method-post)'
export const fmtTime = (ts: string) => new Date(ts).toLocaleTimeString()
