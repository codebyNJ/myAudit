// Builds a static OpenAPI 3 spec from a route list — used for the myIntern
// "Swagger" preview and as a fallback. In the running Fastify app, prefer
// @fastify/swagger, which derives the spec from each route's JSON schema.

export function buildOpenApi({ title = 'API', version = '1.0.0', routes = [] }) {
  const paths = {}
  for (const r of routes) {
    const item = paths[r.path] || (paths[r.path] = {})
    item[r.method.toLowerCase()] = {
      summary: r.summary || '',
      responses: { 200: { description: 'OK' } },
    }
  }
  return { openapi: '3.0.0', info: { title, version }, paths }
}
