// ElevenLabs text-to-speech adapter — the reference "capability above the
// template". The API key comes from the environment (never hardcoded); fetch is
// injectable so it is testable offline.

export async function tts({
  text,
  voiceId,
  apiKey = process.env.ELEVENLABS_API_KEY,
  fetchImpl = fetch,
  baseUrl = 'https://api.elevenlabs.io',
}) {
  if (!apiKey) throw new Error('ELEVENLABS_API_KEY is required (set it in the environment)')
  if (!voiceId) throw new Error('voiceId is required')

  const res = await fetchImpl(`${baseUrl}/v1/text-to-speech/${voiceId}`, {
    method: 'POST',
    headers: { 'xi-api-key': apiKey, 'content-type': 'application/json' },
    body: JSON.stringify({ text, model_id: 'eleven_multilingual_v2' }),
  })
  if (!res.ok) throw new Error(`elevenlabs tts failed: ${res.status}`)
  return await res.arrayBuffer()
}
