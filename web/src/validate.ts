import type { ZodType } from 'zod'

/**
 * Validate a decoded response body against a schema.
 *
 * Policy, chosen deliberately: **throw in dev, warn and fall back in prod.**
 *
 * A schema must never become a new way to blank the page. This app has already
 * shipped one blank screen today — a react/react-dom version skew that crashed
 * at mount — and a validation layer that turns a renamed backend field into a
 * dead UI would be the same failure with better intentions. So in production a
 * rejected payload logs the exact issue path and the caller keeps its previous
 * value; in development it throws, because drift you cannot see is drift you
 * will ship.
 */
export function parsed<T>(schema: ZodType<T>, data: unknown, label: string, fallback: T): T {
  const result = schema.safeParse(data)
  if (result.success) return result.data

  const detail = result.error.issues
    .slice(0, 5)
    .map((i) => `${i.path.join('.') || '<root>'}: ${i.message}`)
    .join('; ')
  const message = `${label} failed schema validation — ${detail}`

  if (import.meta.env.DEV) throw new Error(message)

  console.warn('[myAudit]', message, { received: data })
  return fallback
}
