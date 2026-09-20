/**
 * Where a ticket's branch actually lives, and how to reach it.
 *
 * A ticket's branch is created in the run's sandbox as soon as the fix commits
 * (`sandbox.CreateBranch`). An identically-named branch only appears in the
 * developer's own clone once Push PR has run. A copy action has to follow the
 * branch rather than assume it, or it hands over a command that fails for every
 * ticket that has not been pushed — which is most of them.
 */
export function branchCommand(branch: string, runId: string, pushed: boolean): string {
  return pushed
    ? `git fetch origin && git checkout ${branch}`
    : `git -C runs/${runId} show ${branch}`
}

/** Commits are addressed in the sandbox until the work reaches your clone. */
export function commitCommand(sha: string, runId: string, pushed: boolean): string {
  return pushed ? `git show ${sha}` : `git -C runs/${runId} show ${sha}`
}

export function shortSHA(sha: string): string {
  return sha.length > 7 ? sha.slice(0, 7) : sha
}
