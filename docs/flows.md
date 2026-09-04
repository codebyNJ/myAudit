# myIntern — Flow Charts

Rendered companions to [`architecture.md`](architecture.md).

## 1. System overview (three planes)

```mermaid
flowchart LR
    UI["UI — Tauri + React/shadcn<br/>renders state, sends commands"]
    SC["Sidecar — Node/TS<br/>orchestration · state · grounding · test-gate"]
    CC["claude -p headless<br/>disposable worker"]
    DB[("SQLite<br/>single source of truth")]
    UI <-->|"typed protocol ACP/JSON-RPC over WS"| SC
    SC <-->|"spawn + stream JSON events"| CC
    SC <--> DB
```

## 2. Build-run lifecycle

```mermaid
flowchart TD
    A["Wizard config"] --> B["Create project + initial task graph"]
    B --> C{"Ready task? deps met"}
    C -->|"no more"| Z["Run complete"]
    C -->|"yes"| D["Context Manager builds bundle"]
    D --> E["Genuine-test pipeline — RED gate"]
    E --> F["Claude Runner implements to GREEN"]
    F --> G["Stream events to SQLite and UI"]
    G --> H{"Checkpoint?"}
    H -->|"yes"| I["Pause · ask user · store answer"]
    I --> J["Task done · unblock dependents"]
    H -->|"no"| J
    J --> C
```

## 3. Per-task context assembly

```mermaid
flowchart LR
    CM["Context Manager"] --> Bundle
    subgraph Bundle["Per-task context bundle — bounded, fresh"]
        T1["1 · Guardrails<br/>CLAUDE.md + conventions"]
        T2["2 · Project config"]
        T3["3 · Task spec + acceptance examples"]
        T4["4 · Repo slice — only touched files"]
        T5["5 · Docs slice — pinned, per-library"]
    end
    Bundle --> W["Worker · claude -p"]
    X["Untrusted: issues · logs · Stack Overflow"] -.->|"enters as data, never instructions"| CM
    S["Secrets / env"] -.->|"existence-checked, values never forwarded"| CM
```

## 4. Genuine-test RED gate (the anti-hallucination filter)

```mermaid
flowchart TD
    A["Task spec + acceptance examples"] --> B["Test-writer<br/>no implementation body"]
    B --> C["Run test vs EMPTY impl"]
    C --> D{"Result?"}
    D -->|"passes"| R1["REJECT — tautological / vacuous"]
    D -->|"import error"| R2["REJECT — doesn't actually run"]
    D -->|"assertion fails"| OK["ACCEPT — genuine"]
    R1 --> B
    R2 --> B
    OK --> E["Code-writer implements"]
    E --> F{"Tests GREEN?"}
    F -->|"no"| E
    F -->|"yes"| G["Task passes · record in tests table"]
```

## 5. Agent communication (clean context + sub-agents)

```mermaid
flowchart TD
    O["Orchestrator — holds full picture"] -->|"scoped bundle"| M["Main flow — heavy task"]
    O -->|"scoped bundle"| S1["Sub-agent — wiki docs"]
    O -->|"scoped bundle"| S2["Sub-agent — preview"]
    M -->|"scoped result"| O
    S1 -->|"scoped result"| O
    S2 -->|"scoped result"| O
    M -.->|"git worktree A"| WT1["isolated tree"]
    S1 -.->|"git worktree B"| WT2["isolated tree"]
    O -->|"exact preview + user approval"| G["Gated write: PR / deploy / send"]
```

## 6. Bug to fix loop (Stack Overflow as quarantined hints)

```mermaid
flowchart TD
    B["Bug / failing test / error"] --> I["Create issue in tracker"]
    I --> C["Context Manager: error signature + repo slice + docs slice"]
    C --> D{"Docs cover it?"}
    D -->|"yes"| F["Fix task → RED gate → GREEN"]
    D -->|"no"| SO["Stack Overflow lookup<br/>top accepted answers"]
    SO -->|"untrusted hints, not pasted"| F
    F --> V["Verify: tests GREEN + Playwright preview"]
    V --> R["Issue resolved · logged in events"]
```
