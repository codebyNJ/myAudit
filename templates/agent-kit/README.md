# @myintern/agent-kit

Optional agent kit that myIntern scaffolds into agentic apps. Provider-agnostic
by design: the agent loop takes an injected `generate` function, so the core has
**no hard dependency on any LLM SDK** and is fully testable offline.

## Pieces

| Module | Purpose |
|---|---|
| `src/agent.js` | `runAgent({ generate, tools, messages })` — tool-calling loop |
| `src/session.js` | `createSessionStore(backend)` — conversation history (Map local; Valkey+Mongo in prod) |
| `src/elevenlabs.js` | `tts({ text, voiceId })` — voice adapter; key from env, fetch injectable |
| `src/providers/vercel.js` | `vercelGenerate(model)` — production adapter over the Vercel AI SDK (`ai`) |

## Wiring (production)

```js
import { openai } from '@ai-sdk/openai'
import { vercelGenerate } from '@myintern/agent-kit/providers/vercel'
import { runAgent } from '@myintern/agent-kit/agent'

const generate = vercelGenerate(openai('gpt-4o'))
const { text } = await runAgent({
  generate,
  tools: { /* name: async (args) => result */ },
  messages: [{ role: 'user', content: 'Book me a table for two.' }],
})
```

Swap the model line for `@ai-sdk/anthropic`, etc. — the rest is unchanged.

## Test

```bash
node --test
```

The self-check mocks the LLM provider and `fetch`, so it runs without network or
installed SDKs. Install deps (`npm install`) only when wiring the real Vercel
provider.

## Security

- LLM and provider keys come from the **environment**, never hardcoded.
- `elevenlabs.tts` requires `ELEVENLABS_API_KEY`; it is sent as a header and
  never logged.
