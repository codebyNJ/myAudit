// Provider-agnostic agent loop. `generate` is injected (a Vercel AI SDK adapter
// in production, a mock in tests), so the loop has no hard LLM dependency.
//
// generate({ messages, tools }) -> { text?, toolCall?: { name, args } }

export async function runAgent({ generate, tools = {}, messages, maxSteps = 6 }) {
  let convo = [...messages]
  for (let step = 0; step < maxSteps; step++) {
    const res = await generate({ messages: convo, tools })
    if (res.toolCall) {
      const tool = tools[res.toolCall.name]
      if (!tool) throw new Error(`unknown tool: ${res.toolCall.name}`)
      const result = await tool(res.toolCall.args)
      convo = [...convo, { role: 'tool', name: res.toolCall.name, content: JSON.stringify(result) }]
      continue
    }
    return { text: res.text ?? '', messages: convo }
  }
  throw new Error('agent exceeded maxSteps without a final answer')
}
