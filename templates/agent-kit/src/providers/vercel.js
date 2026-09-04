// Production LLM adapter over the Vercel AI SDK. Kept isolated so the agent core
// and its tests have no hard dependency on `ai`. Wire it as the `generate`
// argument to runAgent.
//
//   import { openai } from '@ai-sdk/openai'        // or anthropic, etc.
//   import { vercelGenerate } from './providers/vercel.js'
//   const generate = vercelGenerate(openai('gpt-4o'))

import { generateText } from 'ai'

export function vercelGenerate(model) {
  return async ({ messages, tools }) => {
    const res = await generateText({ model, messages, tools })
    const call = res.toolCalls?.[0]
    return {
      text: res.text,
      toolCall: call ? { name: call.toolName, args: call.args } : undefined,
    }
  }
}
