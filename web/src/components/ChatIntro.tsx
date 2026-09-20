import { useAppStore } from '../store/slices'

/** Shown before the first message; same wording in the dock and the tab. */
export function ChatIntro() {
  const runs = useAppStore((s) => s.runs)
  const runId = useAppStore((s) => s.runId)
  const project = (runs || []).find((r) => r.id === runId)?.project

  return (
    <div className="agent-reply">
      {project
        ? <>Auditing <b>{project}</b>. Ask me anything about the code — flows, a specific file, why a finding matters.</>
        : <>Import a repo and I'll help you interrogate it here.</>}
    </div>
  )
}
