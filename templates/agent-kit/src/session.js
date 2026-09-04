// Conversation session store. The default backend is an in-memory Map for local
// dev; production injects a Valkey (short-lived) + Mongo (durable) backend that
// implements the same get/set/delete surface.

export function createSessionStore(backend = new Map()) {
  return {
    async append(id, message) {
      const current = backend.get(id) || []
      current.push(message)
      backend.set(id, current)
      return current
    },
    async history(id) {
      return backend.get(id) || []
    },
    async reset(id) {
      backend.delete(id)
    },
  }
}
