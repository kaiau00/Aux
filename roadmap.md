# Aux Product Roadmap

## Executive Summary

Aux should become the coding agent that gets cheaper, faster, and more capable the longer it works on a project.

Aux is not trying to win by adding the most model providers, copying every feature from other coding agents, or putting another chat interface around a shell. Its opportunity is to build a persistent intelligence layer between a developer, their projects, and the single model they already want to use.

The product vision is built around four connected systems:

1. **Project Brain** — a living, project-specific model of architecture, workflows, decisions, skills, and experience.
2. **Context OS** — a token-efficient memory hierarchy that loads only the information needed for the current step.
3. **Cost Governor** — a one-key efficiency layer that controls context, exploration, caching, and validation for the user's preferred model.
4. **Experience Compiler** — a learning loop that turns successful work and user corrections into reusable memories, skills, validation rules, and cost-efficiency policies.

Together, these systems create a compounding loop:

```text
Understand the project
        ↓
Compile the smallest useful task context
        ↓
Optimize the chosen model's context and budget
        ↓
Execute and validate the change
        ↓
Learn reusable project experience
        ↓
Make the next task cheaper and better
```

The core promise is:

> **Aux understands how your project works, gives your preferred model only what it needs, and improves every time you use it.**

Possible positioning lines:

- **The coding agent that learns your codebase.**
- **Spend less context. Ship better code.**
- **Project intelligence for coding agents.**
- **Aux gets cheaper and more capable with every task.**
- **One API key. One model. A project brain that keeps getting better.**

### The one-key product contract

Aux should be exceptionally easy to adopt:

```text
Install Aux
    ↓
Add one API key
    ↓
Choose one preferred model
    ↓
Open a project and start working
```

The user should not need to assemble a fleet of providers, compare models for individual actions, or understand internal orchestration. Aux's sophistication should reduce setup and cost, not create another configuration problem.

### The two-phase product strategy

Aux should be delivered as two connected top-level phases:

1. **Phase 1 — Project intelligence and execution platform.** Build the Project Brain, profiles, Task Compiler, Context OS, persistent memory, Change Impact Engine, Cost Governor, Experience Compiler, validation system, checkpoints, cross-project intelligence, and Optimization Lab already described in this roadmap.
2. **Phase 2 — Visual product experience.** Turn those capabilities into a coherent, unmistakably Aux interface: a focused TUI workbench for execution and a browser workspace for understanding, comparison, and optimization.

Phase 2 is not a cosmetic reskin. It determines whether users can understand what Aux is doing, what it changed, whether the work is validated, what context is active, and what the task cost. The visual layer should be designed alongside Phase 1's data contracts, then implemented as the second top-level product phase once those states are real and inspectable.

## Strategic Priority Order

The roadmap contains many valuable ideas, but they are not equally important. Aux should be judged first on whether it solves the recurring problems developers already experience, then on whether its learning systems create a durable advantage.

### Priority 1: The product people immediately want

These features form the core product and should receive most near-term investment:

1. **Automatic Project Brain and profiles** — open a repository and Aux already understands its structure, commands, instructions, skills, and current state.
2. **Useful persistent memory** — remember project procedures, prior approaches, corrections, and unfinished work without loading an ever-growing memory file.
3. **Context OS** — stop paying to resend unchanged files, raw tool output, old history, unused skills, and irrelevant schemas.
4. **Task Compiler** — turn a short request into the relevant workspace, symbols, memories, skills, validation plan, and context budget.
5. **Change Impact Engine** — identify the files, callers, tests, generated artifacts, and related packages affected by a change.
6. **One-Key Cost Governor** — make the user's chosen model efficient through context, caching, exploration, and validation budgets.

These capabilities support a simple user-visible promise:

> **Aux remembers the project and avoids paying to rediscover it.**

### Priority 2: The compounding moat

Build these after the core product generates reliable structured task history:

1. **Experience Compiler** — turn repeated successful workflows into evaluated skills.
2. **Trajectory Optimizer** — detect and eliminate repeated reads, searches, commands, context thrashing, and unnecessary work.
3. **Optimization Lab** — replay real tasks to improve prompts, retrieval, skills, and Cost Governor policies.
4. **Revision-aware memory evolution** — consolidate, invalidate, and promote knowledge as the repository changes.

These systems make the tenth similar task cheaper and better than the first.

### Priority 3: Expansion features

These are valuable after Aux is excellent within one project:

- Cross-project intelligence.
- Worktree-backed parallel tasks.
- Efficient specialist subagents.
- Speculative context prefetching.
- Organization-wide profiles and skill packs.
- Runtime adapters and ecosystem integrations.
- Advanced browser experimentation and ecosystem tools.

Aux should not allow Priority 3 features to delay or dilute the Project Brain, persistent memory, Context OS, Task Compiler, Change Impact Engine, or Cost Governor.

## What Aux Should Be

Aux should be a local-first project intelligence and execution system with a terminal interface and browser control plane.

It should feel less like starting a new chat and more like returning to an engineer who already knows:

- How the repository is organized.
- Which package owns a feature.
- Which commands are used for development and validation.
- Which files are generated.
- Which tests matter for a particular change.
- Which approaches previously worked or failed.
- Which project skills apply to the current task.
- Which context and validation are worth spending the task budget on.
- What has changed since the last session.

Aux should separate project intelligence from model intelligence. Models will improve and change. The durable value belongs in the project graph, memory, skills, task history, cost data, and evaluation results that Aux maintains locally.

## Why Aux Can Be Better

Most coding agents begin each task with some combination of static instruction files, broad repository search, conversation history, and a large model. They may remember a few facts or load project-specific skills, but the user still pays repeatedly for rediscovery, repeated tool output, stale context, oversized prompts, and unnecessary exploration.

Aux can be better because it will:

1. **Remember the project structurally**, not just as a growing Markdown file.
2. **Compile context per step**, rather than dragging the entire session through every model call.
3. **Treat tool results and source files as addressable memory**, not permanent prompt text.
4. **Make one chosen model more efficient**, rather than requiring users to manage multiple models or API keys.
5. **Learn procedures from successful trajectories**, not just save conversational facts.
6. **Measure its own effectiveness**, using replayable real-project tasks.
7. **Improve across sessions and related repositories**, creating compounding value competitors cannot provide in a stateless session.

The differentiator is not any single feature. It is the integrated learning and efficiency loop.

## Product Principles

### 1. The context window is expensive working memory

Source files, tool schemas, old commands, logs, and completed reasoning should not remain in every prompt by default. Aux should maintain a memory hierarchy and load information on demand.

### 2. Profiles are compiled, not dumped

A project profile may contain thousands of facts and many skills. The model should receive a small task-specific projection, not the whole profile.

### 3. Project knowledge must compound

Every successful task should leave the project brain more useful than it was before the task began.

### 4. Use deterministic systems where possible

LSP, AST analysis, dependency graphs, git, test results, content hashes, and file watchers should do work that does not require an LLM.

### 5. Make the preferred model efficient

Aux should assume the user has selected one primary model and one API key. It should reduce that model's workload through deterministic code intelligence, context paging, caching, artifact storage, precise retrieval, and targeted validation. Optional same-provider economy features may exist later, but the core product must never depend on multiple models.

### 6. Optimize accepted outcomes, not activity

More tool calls, more agents, more tokens, and longer reasoning are not inherently better. Aux should optimize for successful changes with minimal cost, latency, and rework.

### 7. The harness must remain replaceable

Aux began from the original Go OpenCode codebase. It should acknowledge that lineage clearly while evolving its distinctive systems behind stable interfaces so the runtime can absorb upstream ideas, new protocols, and new models without rewriting the product.

## The Product Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                         Aux Clients                         │
│                  TUI · Browser · CLI · API                  │
└─────────────────────────────┬───────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                     Task and Agent Runtime                  │
│       Task compiler · Planner · Executor · Validation       │
└───────────┬─────────────────┬─────────────────┬─────────────┘
            │                 │                 │
┌───────────▼──────┐ ┌────────▼────────┐ ┌──────▼─────────────┐
│   Project Brain  │ │    Context OS   │ │   Cost Governor    │
│ graph · profiles │ │ pages · paging  │ │ budget · caching   │
│ memory · skills  │ │ cache · budgets │ │ depth · validation │
└───────────┬──────┘ └────────┬────────┘ └──────┬─────────────┘
            │                 │                 │
┌───────────▼─────────────────▼─────────────────▼─────────────┐
│                    Local Event and Data Layer               │
│  SQLite · content store · trajectories · metrics · evals    │
└─────────────────────────────────────────────────────────────┘
```

The architecture should be event-driven. Every prompt, context selection, page load, tool call, model call, edit, test result, validation result, and user correction should be represented as a structured event. That enables replay, evaluation, optimization, debugging, and alternate clients.

## Product Pillar 1: Project Brain and Profiles

### Project profiles

Every repository should have a persistent profile created and maintained by Aux. A profile should include:

- Project identity and repository root.
- Languages, frameworks, and package managers.
- Workspace and package boundaries.
- Important entry points and architectural layers.
- Symbol and dependency graph metadata.
- Build, test, lint, format, release, and development commands.
- Generated files and their generators.
- Project instructions and conventions.
- Available MCP servers, tools, and skills.
- Preferred provider, primary model, and cost-governor defaults.
- Known task categories.
- Related repositories and external services.
- Project memories and prior task experience.
- Current git branch, revision, and dirty state.

Profiles should be automatically discovered, editable, exportable, and versionable.

### Hierarchical profiles

Profiles should compose rather than duplicate information:

```text
User profile
└── Organization profile
    └── Repository profile
        ├── Workspace or package profile
        ├── Branch profile
        └── Current task profile
```

Examples:

- A user profile stores the preferred provider, primary model, response style, and common personal skills.
- An organization profile stores shared engineering workflows and reusable skills.
- A repository profile stores architecture and top-level commands.
- A package profile stores package-specific conventions and test commands.
- A branch profile stores temporary decisions for work in progress.
- A task profile contains only what the active task needs.

### Automatic profile construction

On first run, Aux should build a profile using deterministic inspection first:

1. Detect repository and workspace structure.
2. Parse build and package configuration.
3. Index symbols and dependencies.
4. locate existing instruction files and skills.
5. Detect common commands from scripts and CI.
6. Read a small set of architectural entry points.
7. Ask a model only to interpret ambiguous structure.
8. Save the result as structured project state.

Subsequent runs should update incrementally from git and filesystem changes rather than rebuilding the profile.

### Project modes

Profiles should expose task modes that activate different combinations of context, skills, tools, validation, and Cost Governor settings:

- `debug`
- `feature`
- `review`
- `migration`
- `performance`
- `frontend`
- `backend`
- `infrastructure`
- `release`
- `documentation`

Aux should infer a likely mode from the task, while allowing explicit selection.

## Product Pillar 2: Context OS

The Context OS is Aux's main token-efficiency engine.

### Memory hierarchy

```text
L1 — Active task, immediate plan, current symbols, latest tool result
L2 — Recently accessed files, decisions, errors, and related symbols
L3 — Project memories, skills, summaries, and stored artifacts
L4 — Full trajectories, complete files, logs, history, and indexes
```

Only L1 is guaranteed to appear in each model call. Other levels are paged in as needed.

### Typed context pages

Aux should store context as typed, content-addressed objects:

```text
file:internal/auth/service.go@<content-hash>
symbol:AuthService.Login@<content-hash>
decision:use-redis-rate-limits
memory:migrations-require-docker
skill:add-api-endpoint@v3
tool-result:test-run-142
artifact:full-build-log-88
plan:task-109/current
```

Each page should have:

- Type.
- Stable identifier.
- Full-fidelity content.
- Compact representation.
- Token estimate.
- Source revision.
- Dependencies.
- Recency and access count.
- Pin and eviction state.

### Demand paging

Models should receive compact page manifests and request full content only when needed.

Example:

```text
Active context pages:
- symbol:AuthService.Login — 180 tokens — loaded
- memory:auth-tests-require-redis — 34 tokens — summary
- tool-result:test-run-142 — 22 tokens — failures only
- file:internal/auth/types.go — archived
```

The runtime should detect page faults when a model needs evicted information, restore the full page, and learn from fault history which pages should remain resident.

### Tool-output virtualization

Large command output should be stored outside the conversation. The model receives:

- Exit status.
- Short summary.
- Relevant warnings or failures.
- A handle to the complete output.

For example:

```text
go test ./...
Result: failed
Packages: 84 passed, 1 failed
Failure: internal/auth TestResetRateLimit
Full output: artifact:test-run-142
```

This applies to build logs, test output, git diffs, search results, API responses, and MCP results.

### Delta-based file context

When the model has already read a file, Aux should avoid sending it again unchanged.

- Same content: return its existing page handle.
- Small change: return only changed regions plus surrounding symbols.
- Large change: create a new page version and invalidate dependent summaries.
- Model requests full fidelity: restore the complete file.

### Lazy tool and skill loading

Large MCP configurations and skill libraries can consume context before useful work begins. Aux should expose a compact catalog first:

```text
Available capabilities:
- github: repository and pull request operations
- browser: local web application interaction
- railway: deployment and infrastructure operations
- add-api-endpoint: project workflow skill
```

Detailed schemas and skill instructions should load only after selection.

### Cache-aware prompt compilation

The prompt compiler should:

- Keep stable content in an identical prefix and deterministic order.
- Separate stable project state from volatile task state.
- Avoid timestamps and nondeterministic text in cached prefixes.
- Track provider-specific prompt-cache behavior.
- Report effective cached and uncached input costs.

### Context budget modes

Users should be able to specify:

- Maximum input tokens.
- Maximum task cost.
- Maximum elapsed time.
- Use only the configured local model endpoint.
- Cheapest successful strategy.
- Maximum-quality strategy.

Aux should allocate the budget across instructions, project context, task history, retrieved code, skills, and tool results.

## Product Pillar 3: Persistent Memory

Memory should be structured into three layers.

### Factual memory

Stable or semi-stable facts about the project:

- Ownership boundaries.
- Architectural relationships.
- Generated file rules.
- Environment requirements.
- Naming and coding conventions.

### Procedural memory

How work is performed successfully:

- Adding a migration.
- Creating an endpoint.
- Updating generated clients.
- Running a local integration environment.
- Releasing a package.

### Episodic memory

Useful information from specific past tasks:

- Approaches that failed and why.
- Unexpected dependency relationships.
- Flaky tests and observed conditions.
- User corrections.
- Performance characteristics.

### Memory lifecycle

Memory should not mean appending endlessly to a file. Aux should:

1. Extract candidate memories from trajectories.
2. Merge duplicates.
3. Separate universal knowledge from project-specific knowledge.
4. Link memories to supporting files, commands, and task outcomes.
5. Track the git revision at which a memory was learned.
6. Invalidate or lower confidence when related code changes.
7. Retrieve only memories relevant to the current task.
8. Periodically consolidate or archive low-value memories.

### Branch-aware memory

Temporary decisions may be correct on one branch and wrong on another. Memories should support:

- Repository-wide scope.
- Package scope.
- Branch scope.
- Task scope.
- User scope.
- Organization scope.

When a branch merges, Aux can promote durable memories and discard temporary ones.

## Product Pillar 4: Experience Compiler

The Experience Compiler converts trajectories into reusable capability.

### Inputs

- Successful task trajectories.
- Failed attempts.
- User corrections.
- Final accepted diffs.
- Validation results.
- Repeated command sequences.
- Primary-model cost and task performance.

### Outputs

- New memories.
- New or improved skills.
- Validation rules.
- Retrieval hints.
- Task templates.
- Context, budget, and validation policies.
- Project-mode updates.

### Skill discovery

Aux should detect repeated workflows. For example:

```text
Repeated endpoint tasks
        ↓
Common files, commands, and validation steps detected
        ↓
Candidate add-api-endpoint skill generated
        ↓
Skill replayed against prior tasks
        ↓
Skill accepted only if results improve
```

### Skill optimization

Skills should be versioned and evaluated. Proposed skill changes should be tested against recorded tasks. Aux should retain changes only when they improve outcome quality, cost, or latency on a held-out set.

This turns skills into optimized external agent state rather than static prompt snippets.

### Learn by demonstration

An `aux learn` workflow should let users demonstrate a process:

1. Start recording.
2. Perform commands and edits normally.
3. Stop recording.
4. Aux identifies inputs, outputs, variable steps, and validation.
5. Aux proposes a reusable skill.
6. The skill is tested against the current project profile.

Good targets include migrations, releases, generated clients, new services, deployment, and integration-test setup.

## Product Pillar 5: Task Compiler

Raw prompts should be compiled into structured task specifications before expensive execution.

Input:

> Add rate limiting to password reset.

Compiled task:

```text
Intent: backend security feature
Workspace: internal/auth
Relevant symbols: ResetPassword, Mailer, RateLimiter
Related tests: auth/reset_test.go
Applicable skills: add-middleware, security-change
Relevant memory: Redis is used for ephemeral counters
Likely validation: auth unit tests and integration tests
Cost mode: balanced
Context budget: 12,000 tokens
```

The compiler should:

- Classify task type.
- Select project and workspace profiles.
- Resolve named files and symbols.
- Select memories and skills.
- Estimate likely impact.
- Determine validation commands.
- Choose an initial Cost Governor policy.
- Set cost and context budgets.
- Generate a structured execution plan.

Task compilation should use deterministic project analysis first, then use the selected primary model only when interpretation is necessary.

## Product Pillar 6: One-Key Cost Governor

Aux should assume one API key and one preferred model. The Cost Governor makes that model cheaper and more effective by controlling how much work, context, exploration, and validation each task receives.

The Cost Governor should present understandable user modes:

| Mode | Behavior |
| --- | --- |
| Efficient | Aggressive paging, targeted exploration, compact tool output, targeted validation |
| Balanced | Default context and validation based on project history |
| Maximum quality | Broader retrieval, more extensive validation, higher task budget |
| Local only | Use the configured local endpoint and no hosted-provider calls |
| Budget capped | Stay beneath a user-specified estimated cost |

Example:

```text
Provider: Anthropic
Primary model: Claude Sonnet
Mode: Efficient
Task budget: $0.25

Estimated allocation:
- Project and task context: 8K tokens
- Implementation loop: 18K tokens
- Validation and recovery: 5K tokens
- Expected cached input: 71%
```

### Cost Governor inputs

- Task type and complexity.
- Project-specific historical cost and success.
- Primary model context limit and pricing.
- Input and output pricing.
- Prompt cache availability.
- Current working-set size.
- Relevant-memory and skill size.
- Estimated exploration depth.
- Expected validation breadth.
- Remaining task budget.
- Prior failed attempts.

### What the Cost Governor controls

- Maximum active context size.
- Retrieval depth and graph radius.
- Number and size of file regions loaded.
- When raw tool output is archived.
- When conversation history is converted into structured state.
- How much speculative prefetching is allowed.
- Whether optional exploratory searches are worthwhile.
- Whether validation runs targeted checks or broader suites.
- When the agent should recompile context after a failed approach.

### Project-specific cost policy

The same nominal task can require different budgets in different projects. Aux should learn policies such as:

- Go changes in this repository usually require a small graph radius and one package test.
- Frontend changes often need component code, shared types, and a browser validation step.
- Database migrations require a broader context set and integration tests.
- This monorepo has expensive global tests, so impacted-package tests should run first.

Cost-policy changes must be evaluated against complete task outcomes. Aux should optimize total cost per accepted change, not simply minimize the tokens used by one call.

### Optional same-key economy mode

If a provider exposes multiple model tiers behind the same API key, Aux may later offer an optional economy mode. It must be automatic, disabled by default, and never required for the Project Brain, Context OS, memory, or Cost Governor to deliver savings. The primary product contract remains one key and one chosen model.

## Product Pillar 7: Change Impact Engine

Aux should use its project graph to estimate the effects of a proposed change before and during implementation.

Questions the engine should answer:

- Which callers depend on this symbol?
- Which tests cover this code?
- Which services consume this event or API?
- Which generated files need updating?
- Which documentation describes this behavior?
- Which project memories may become stale?
- What is the smallest relevant test set?
- Which related repositories may be affected?

The impact engine should combine:

- AST and symbol relationships.
- LSP references.
- Import and dependency graphs.
- Test coverage when available.
- Git co-change history.
- Build-system metadata.
- Project memories.

This supports more precise context selection and faster validation.

## Product Pillar 8: Speculative Context Prefetching

While a model is planning, Aux can prepare likely next context locally:

- Definitions of mentioned symbols.
- Callers and callees.
- Nearby tests.
- Configuration controlling the feature.
- Recent commits touching the same area.
- Applicable memories and skills.

Prefetched pages are not inserted into the prompt until requested. This reduces tool latency without paying unnecessary token costs.

Aux should learn prefetch patterns from prior trajectories and measure both hit rate and wasted work.

## Product Pillar 9: Efficient Multi-Agent Work

Multi-agent behavior should be used for isolation and context efficiency, not as a feature-count exercise.

### Structured subcontexts

Each subagent should receive:

- A bounded task.
- A selected project-profile slice.
- Only relevant tools and skills.
- Its own context budget.
- A structured output contract.

The parent should receive a concise result delta, not the complete child conversation.

### Shared context pages

Subagents working in the same project should reference the same content-addressed pages. Source files, build logs, and project summaries should not be duplicated into every context.

### Useful specialist roles

- Explorer.
- Planner.
- Implementer.
- Test selector.
- Debugger.
- Reviewer.
- Documentation updater.

Aux should spawn them only when historical data suggests the additional calls improve the task's cost-quality tradeoff.

## Product Pillar 10: Validation and Proof of Done

Validation is also an efficiency feature: it reduces retries and human rework.

Every implementation task should produce a compact result artifact containing:

- Requirements addressed.
- Files changed and the role of each change.
- Tests and checks run.
- Exact validation outcomes.
- Remaining failures.
- Unverified assumptions.
- Scope changes.
- Final cost, latency, and model usage.

Aux should connect requirements to changes and validation:

```text
Requirement
    └── implementation change
            └── test, diagnostic, or runtime evidence
```

This proof can be reused in the TUI, dashboard, commit message, pull request description, and future project memory.

## Product Pillar 11: Trajectory Optimization

Aux should analyze work while it is happening for efficiency problems:

- Repeatedly reading unchanged files.
- Running identical searches.
- Repeating the same failed command.
- Consuming tokens without producing a plan or diff.
- Expanding scope unnecessarily.
- Spending primary-model tokens on mechanical output that Aux could compact deterministically.
- Loading context that is never referenced again.
- Repeatedly evicting and reloading the same page.
- Running a larger test suite than the impact graph requires.

Responses can include:

- Deduplicating calls automatically.
- Tightening or expanding the task budget.
- Recompiling context.
- Pinning frequently faulted pages.
- Summarizing or archiving stale state.
- Revising the task plan.
- Replanning a difficult step with a refreshed working set.

The goal is a runtime optimizer for agent trajectories.

## Product Pillar 12: Checkpoints, Branching, and Reuse

Aux already has file-history foundations. They should become a complete state system:

- Automatic checkpoint before each edit group.
- Restore code, conversation state, or both.
- Fork a session from any checkpoint.
- Compare two approaches from a shared starting point.
- Run parallel approaches in isolated git worktrees.
- Reuse a successful plan on a similar task.

Context state should form a DAG so branches share common pages instead of duplicating full histories.

## Product Pillar 13: Cross-Project Intelligence

Aux profiles should link related repositories and services:

```text
Product
├── web application
├── backend API
├── shared SDK
├── infrastructure
└── documentation
```

The project brain should know:

- Which frontend consumes which backend API.
- Which client is generated from which schema.
- Which infrastructure variables support a service.
- Which repositories deploy together.
- Which shared skills apply across the product.

This enables coordinated multi-repository tasks without loading all repositories into every model call.

## Product Pillar 14: Local Optimization Lab

Aux should turn recorded sessions into replayable evaluations.

The lab should compare combinations of:

- Cost Governor settings for the configured primary model.
- Prompts.
- Retrieval strategies.
- Context budgets.
- Skills.
- Memory configurations.
- Cost Governor policies.
- Single-agent and multi-agent strategies.

Example report:

```text
Task: Fix authentication timeout

Chosen model + full context         $1.12   success   94s
Chosen model + basic compaction     $0.71   success   88s
Chosen model + Context OS           $0.43   success   81s
Chosen model + aggressive budget    $0.28   failed    73s
```

The lab should support:

- Replay using saved repositories or fixtures.
- Held-out task sets.
- Regression detection.
- Cost and latency comparison.
- Retrieval precision and recall.
- Skill version comparison.
- Cost-policy promotion only after measurable improvement.

This is how Aux can optimize itself for each project instead of asking users to guess at settings.

## Product Pillar 15: TUI and Browser Control Plane

Aux's visual direction should be a **warm technical workbench**: neutral charcoal and warm-black surfaces, ivory text, restrained amber focus, and unambiguous semantic colors. Amber remains the recognizable brand color, but it should not also represent every border, status, graph, warning, success, and error.

The interface must answer five questions at a glance:

1. What is Aux doing?
2. What has it discovered?
3. What has it changed?
4. Has it verified the work?
5. How much context, time, and money has it used?

### TUI

The TUI should remain the fastest interface for direct work and should feel like an active task workspace rather than a raw scrolling transcript:

- Start and resume tasks.
- Select project, profile, and mode.
- See the active task, plan step, and execution state.
- Group mechanical activity into collapsible Searching, Reading, Editing, and Testing steps.
- Keep changed files and validation status visible without scrolling through tool history.
- See the active context working set as a token budget composition.
- Inspect cost, cache use, and active budget.
- Switch between task branches.
- Invoke project skills.
- Expand commands, tool results, and reasoning only when detail is needed.
- Adapt gracefully to narrow terminals by collapsing secondary panes into drawers.
- Present a distinct composer with attachment state, send/cancel state, and contextual shortcuts.

### Browser control plane

The local dashboard should evolve from a read-only event viewer into a rich project and task interface. Its hierarchy should prioritize the active task, activity timeline, changed files, validation, context, and cost before aggregate session telemetry:

- Project brain explorer.
- Architecture and dependency graph.
- Memory browser.
- Skill library and evaluation history.
- Context-page residency and token use.
- Cost Governor and context-budget timeline.
- Cost and latency breakdown.
- Active agents and task tree.
- Validation results.
- Session comparisons.
- Optimization-lab experiments.

Decorative visualization should earn its space. The current ornamental Live Core should either become a real task-state visualization or be replaced by useful active-task information. Likewise, large lifetime session/token cards should not be more prominent than the current outcome.

### Visual system

- Use approximately 90% neutral surfaces, 8% muted structure, and 2% bright brand emphasis.
- Reserve bright amber for focus, active execution, and the Aux mark.
- Use true green for passing validation, red for failures, yellow for warnings, and blue/cyan for informational context states.
- Reduce grids, glow, gradients, and repeated panel borders so important states carry more contrast.
- Define shared spacing, typography, border, icon, focus, and status tokens across TUI and browser implementations.
- Prefer labels plus icons; never rely on color alone.
- Use motion only for live activity, transitions, and state changes, with reduced-motion support in the browser.
- Give empty, loading, disconnected, failed, and narrow-layout states intentional designs.

The TUI is the cockpit for execution. The browser is the workspace for understanding and optimization.

## Token and Cost Reduction Strategy

Aux should attack cost in this order:

### 1. Eliminate mechanical prompt waste

- Virtualize raw tool output.
- Deduplicate repeated source content.
- Remove obsolete metadata.
- Archive completed work.
- Load tool schemas lazily.

### 2. Improve context selection

- Compile task-specific profiles.
- Retrieve symbols and regions instead of whole files.
- Use graph and LSP relationships.
- Track which context actually contributes to outcomes.

### 3. Maximize caching

- Stable prefixes.
- Content-addressed pages.
- Shared pages across tasks and agents.
- Provider-specific cache accounting.

### 4. Govern one model intelligently

- One API key and one preferred model by default.
- Context and exploration budgets matched to the task.
- Provider-cache-aware prompt assembly.
- Project-specific cost and success history.

### 5. Avoid repeated work

- Persistent project understanding.
- Procedural memory.
- Reusable skills.
- Impact-aware test selection.
- Trajectory deduplication.

### 6. Improve through offline learning

- Replay tasks.
- Optimize skills.
- Tune Cost Governor policies.
- Compare retrieval strategies.

## Two-Phase Roadmap

### Phase 1: Project Intelligence and Execution Platform

Phase 1 builds the useful, compounding product we have already defined. Its milestones are ordered by dependency and learning value. Aux should avoid building advanced autonomous learning before it can measure whether the basic systems improve outcomes.

#### Phase 1.0: Measurement and Runtime Foundation

**Goal:** Make every important part of an Aux task measurable and replayable.

Deliverables:

- Define a structured event schema for model calls, context, tools, edits, validation, and user corrections.
- Persist semantic-retrieval metrics currently available only in memory.
- Record input tokens, output tokens, cached tokens, cost, and latency per model call.
- Record context pages or file regions supplied to each call.
- Record which retrieved files are later changed, cited, or used for validation.
- Build a basic session replay format.
- Separate core agent events from TUI rendering.
- Establish explicit upstream attribution and product lineage documentation.

Exit criteria:

- A complete session can be replayed and inspected without relying on terminal output.
- Aux can calculate total cost and latency per task and per step.
- Retrieval strategies can be compared on the same task fixture.

#### Phase 1.1: Project Profiles and Task Compiler

**Goal:** Give Aux persistent structured awareness of each project.

Deliverables:

- Simple one-key onboarding and explicit primary-model selection.
- Project profile schema and storage.
- Automatic language, framework, workspace, and command detection.
- Profile activation by working directory.
- User, repository, workspace, branch, and task profile inheritance.
- Project mode selection and inference.
- Task compiler producing structured task specifications.
- Profile view in the dashboard.
- Import of existing Aux, AGENTS.md, CLAUDE.md, Cursor, and compatible skill instructions.

Exit criteria:

- Starting the same task type in a known project requires less discovery than a first-time session.
- The compiled task profile is materially smaller than loading all project instructions and context.
- Users can switch among projects without manually reconfiguring tools and skills.

#### Phase 1.2: Context OS and Token Efficiency

**Goal:** Reduce repeated input tokens without reducing task success.

Deliverables:

- Typed context-page store.
- Content-addressed file and symbol pages.
- Tool-output artifact storage.
- Compact tool-result digests.
- Delta-based rereads.
- Page manifests and demand loading.
- Pinning, eviction, and page-fault tracking.
- Lazy MCP schema and skill loading.
- Stable prompt-prefix compiler.
- Per-task context budget controls.
- Real context-pane operations backed by runtime state.

Exit criteria:

- Tool-heavy sessions show a significant reduction in uncached input tokens.
- Repeated file reads rarely resend unchanged content.
- Context paging does not reduce success on the replay suite.
- The context pane accurately represents the active model working set.

#### Phase 1.3: Persistent Memory and Impact Engine

**Goal:** Make project knowledge compound across sessions.

Deliverables:

- Factual, procedural, and episodic memory types.
- Memory extraction from successful and failed tasks.
- Retrieval by task, symbol, workspace, and project mode.
- Revision-aware memory invalidation.
- Branch-scoped memory.
- Memory consolidation and deduplication.
- Hybrid impact graph using AST, LSP, git, and build metadata.
- Related-test and affected-symbol suggestions.
- Memory and graph explorer in the dashboard.

Exit criteria:

- Aux reuses prior project knowledge in relevant tasks without loading the entire memory store.
- Stale memories are detected when supporting code changes.
- Impact-based test selection reduces validation time while preserving failure detection on evaluation tasks.

#### Phase 1.4: Cost Governor and Trajectory Optimizer

**Goal:** Reduce the chosen model's total task cost without requiring additional API keys or models.

Deliverables:

- Budget modes for cost, time, and quality.
- Context, retrieval, exploration, and validation allocations.
- Cache-aware effective-cost calculation.
- Repeated-call and repeated-output detection.
- Context thrashing detection.
- Cost and success history by project and task type.
- Cost Governor visualization.
- Optional same-key economy mode only where a provider supports it.

Exit criteria:

- Governed execution costs less than a full-context baseline using the same model.
- Success rate remains within an agreed tolerance of the maximum-quality mode.
- The Cost Governor learns at least one project-specific policy that beats the global default.

#### Phase 1.5: Experience Compiler and Self-Improving Skills

**Goal:** Turn completed work into reusable, evaluated procedures.

Deliverables:

- Candidate-skill extraction from repeated trajectories.
- Skill versioning.
- Skill replay and held-out evaluation.
- Automatic promotion and rollback based on measured results.
- `aux learn` demonstration workflow.
- Cross-project skill classification.
- Project-specific retrieval and validation hints learned from experience.
- Experience Compiler view in the dashboard.

Exit criteria:

- At least one generated skill improves cost, latency, or success on repeated project tasks.
- Skill changes are never promoted without replay evidence.
- Repeated workflows require fewer tokens and fewer exploratory tool calls over time.

#### Phase 1.6: Cross-Project and Parallel Intelligence

**Goal:** Coordinate work across repositories and isolated task branches.

Deliverables:

- Related-project graph.
- Cross-repository profile composition.
- Worktree-backed parallel tasks.
- Shared context pages across subagents.
- Structured subagent result deltas.
- Context DAG for session branching and reuse.
- Multi-repository impact analysis.
- Product-level profiles spanning services, clients, infrastructure, and documentation.

Exit criteria:

- Aux can perform a coordinated change across related repositories without loading all repositories into every model call.
- Parallel agents do not duplicate large context payloads.
- Branches share common context history and store only deltas.

#### Phase 1.7: Optimization Lab and Ecosystem

**Goal:** Make Aux an adaptive platform rather than a fixed harness.

Deliverables:

- Local experiment runner.
- Primary-model, skill, retrieval, prompt, and Cost Governor comparisons.
- Regression dashboards.
- Shareable project-profile templates.
- Agent Skills compatibility.
- Lifecycle hooks.
- Stable local API and event protocol.
- Optional adapters for other coding-agent runtimes.
- Organization-wide profiles and skill packs.

Exit criteria:

- Users can empirically choose the best context and cost strategy for their configured model.
- New runtime or model integrations do not require rewriting the Project Brain or Context OS.
- Aux improvements can be evaluated before becoming defaults.

### Phase 2: Visual Product Experience

**Goal:** Make Aux's usefulness immediately legible and give it a distinctive, calm, developer-focused visual identity across the TUI and browser dashboard.

Phase 2 should expose the real state produced by Phase 1 rather than simulate intelligence with decorative UI. The visual experience should make active work, changes, validation, context, and cost understandable at a glance while keeping raw detail one action away.

#### Phase 2.1: Shared Visual Foundation

Deliverables:

- Define the warm technical workbench direction and a cross-surface design-token specification.
- Refine the Aux dark and light palettes around neutral surfaces, ivory text, restrained amber brand emphasis, and distinct semantic colors.
- Define typography, spacing, border, focus, icon, elevation, animation, and data-visualization rules.
- Create shared names and state semantics for queued, active, completed, failed, blocked, cancelled, stale, pinned, cached, and validated.
- Audit contrast, color-only communication, terminal color fallbacks, keyboard focus, browser reduced motion, and text truncation.
- Create representative visual fixtures for empty, active, tool-heavy, edit-heavy, validating, failed, completed, disconnected, and narrow-screen states.

Exit criteria:

- The TUI and dashboard use the same information hierarchy and state language.
- Amber is a distinctive focus/brand color rather than the color of every element.
- Every important state remains understandable without color.

#### Phase 2.2: TUI Workbench

Deliverables:

- Replace transcript-first hierarchy with a task workspace centered on objective, progress, changes, validation, and final outcome.
- Group low-level tool traffic into collapsible Searching, Reading, Editing, and Testing activity rows.
- Add a persistent compact header for project, branch, task, model, cost, and context usage.
- Add a persistent change surface showing modified files and addition/removal counts.
- Add a persistent validation surface with pending, running, passing, failing, blocked, and unverified states.
- Redesign the composer as a framed input surface with clear focus, attachments, send/cancel state, and contextual shortcuts.
- Replace the full wrapped dashboard URL with a compact `Dashboard` action and connection indicator.
- Improve focus styling, selection, scrolling position, loading placeholders, errors, confirmations, and permission prompts.
- Add responsive terminal breakpoints: full multi-pane workbench, compact two-pane view, and single-pane drawer navigation.
- Preserve direct keyboard operation and keep tool/reasoning details expandable.

Exit criteria:

- A user can identify current activity, changed files, validation state, context usage, and cost without searching the transcript.
- Mechanical tool output no longer dominates routine sessions.
- The task remains usable at agreed narrow terminal widths.
- Focus and available actions are visually obvious.

#### Phase 2.3: Signature Context Visualization

Deliverables:

- Replace the simple file list with context budget composition by task/plan, project knowledge, active code, tool results, memory/skills, validation, and recent conversation.
- Show resident, available, pinned, evicted, stale, rejected, and faulted context states.
- Show token estimates, cached versus uncached context, selection reasons, and savings from artifacts/deduplication.
- Allow users to inspect, pin, exclude, or reload pages through real runtime operations.
- Add context-thrashing and budget-pressure warnings that explain the problem without overwhelming the main task view.

Exit criteria:

- The visualization reconciles with the actual compiled prompt manifest.
- Users can understand what consumes context and why an item is present.
- Context controls change runtime state rather than only local UI decoration.

#### Phase 2.4: Browser Workspace

Deliverables:

- Replace the current equal-panel dashboard with an active-task-first hierarchy.
- Promote current task, task stage, changed files, validation, context composition, cost, and activity timeline.
- Move aggregate session totals into a compact analytics summary.
- Replace or repurpose the decorative Live Core with a meaningful task-state or remove it.
- Add expandable execution events, diff summaries, validation evidence, and raw logs.
- Add project/profile, memory, skill, graph, Cost Governor, and experiment views as Phase 1 data becomes available.
- Add responsive navigation and layouts for laptop, tablet, and narrow browser widths.
- Design intentional loading, no-session, disconnected, stale-data, permission, redacted-content, and error states.

Exit criteria:

- The dashboard communicates the active task outcome before historical telemetry.
- Every visual metric traces to a durable runtime value.
- The browser complements the TUI instead of duplicating it.

#### Phase 2.5: Polish, Usability, and Visual Validation

Deliverables:

- Run task-based usability sessions for starting work, understanding activity, reviewing changes, diagnosing failure, and confirming completion.
- Add screenshot/golden tests for stable TUI states and browser visual regression tests at standard breakpoints.
- Test common terminal themes, 16/256/true-color modes, light/dark browser modes, zoom levels, and reduced motion.
- Measure time to identify task state, changed files, validation result, cost, and context pressure.
- Tune animation, density, truncation, scroll behavior, and progressive disclosure from observed confusion.
- Produce a consistent Aux mark, favicon, terminal symbol, README imagery, and empty-state treatment.

Exit criteria:

- Representative users can answer the five core interface questions without guidance.
- Visual regression coverage exists for critical states and responsive layouts.
- Accessibility and terminal compatibility checks pass.
- Aux has a recognizable visual identity without sacrificing information clarity.

## Recommended Phase 1 First Implementation Slice

The first end-to-end slice should prove the central thesis without attempting the entire roadmap.

### Slice: Known-project task compilation

1. Add a structured project-profile record.
2. Detect repository languages, packages, scripts, CI commands, and instruction files.
3. Store a compact architecture manifest.
4. Compile each user request into a task profile.
5. Record exactly which profile entries and files enter each model call.
6. Store large tool results as artifacts and return compact digests.
7. Compare tokens and task outcomes against the current agent loop.
8. Run both the baseline and optimized path with the same API key and primary model.

This slice connects profiles, measurement, task compilation, and token reduction. It will reveal which parts deserve deeper investment.

## Success Metrics

### Primary metric

**Accepted, validated changes per dollar of model cost.**

### Supporting metrics

- First-pass task success.
- First-pass validation success.
- Input tokens per accepted change.
- Uncached input tokens per accepted change.
- Output tokens per accepted change.
- Model cost per accepted change.
- Wall-clock time per accepted change.
- Number of exploratory tool calls.
- Repeated read and repeated command rate.
- Context-page fault rate.
- Context-page thrashing rate.
- Retrieval precision: selected context that proved useful.
- Early retrieval recall: relevant files found before editing.
- Relevant-memory hit rate.
- Skill reuse rate.
- Skill improvement on held-out tasks.
- Cost Governor savings against a same-model full-context baseline.
- Human correction volume.
- Reverted or abandoned task rate.

### Compounding metrics

Aux should demonstrate improvement as project experience grows:

- Cost of the first task of a type versus the tenth.
- Discovery calls for the first task versus later tasks.
- Percentage of tasks using learned memories or skills.
- Percentage of project knowledge retrieved rather than rediscovered.
- Improvement from project-specific cost policies.

### Phase 2 visual and usability metrics

- Time to identify the active task operation.
- Time to identify changed files.
- Time to determine validated, failed, blocked, or unverified state.
- Time to find current cost and context pressure.
- Percentage of routine tasks understandable without expanding raw tool output.
- Accidental key actions and focus/navigation errors.
- Successful use at wide, medium, and narrow terminal breakpoints.
- Dashboard task-state comprehension before viewing raw events.
- Accessibility, contrast, keyboard, reduced-motion, and terminal-color compatibility results.
- Visual-regression coverage across critical task states.

## Competitive Positioning

### Versus generic terminal agents

Generic agents provide models, tools, sessions, skills, and shell access. Aux adds a persistent project intelligence layer and optimizes the complete trajectory over time.

### Versus IDE-centric agents

IDE agents benefit from editor context and indexes. Aux should provide deeper cross-session memory, model independence, terminal-native execution, project-profile composition, and measurable cost optimization.

### Versus cloud coding agents

Cloud agents are strong at background issue-to-pull-request workflows. Aux should excel at local project understanding, iterative developer collaboration, related repositories, local models, and reusable project experience.

### Versus static project instruction files

Static files load broadly and require manual maintenance. Aux profiles are structured, hierarchical, revision-aware, task-compiled, and evaluated.

### Versus ordinary agent memory

Ordinary memory stores facts. Aux memory separates facts, procedures, and episodes, connects them to the project graph, invalidates them when code changes, and converts repeated experience into skills.

### Versus multi-model orchestration

Multi-model orchestration can lower the price of individual calls, but it makes onboarding, credentials, behavior, and debugging more complicated. Aux should deliver most of its savings by making one preferred model dramatically more efficient through project memory, context paging, caching, precise retrieval, and targeted validation.

## Features That Are Necessary but Not the Moat

Aux should support these capabilities, but should not define itself by them:

- Compatibility with common providers.
- MCP.
- Agent Skills.
- Hooks.
- Custom commands.
- Subagents.
- Checkpoints.
- Worktrees.
- GitHub integrations.
- A client/server API.
- A polished TUI.

These are increasingly expected features. They should serve the Project Brain, Context OS, and Experience Compiler.

## Things Aux Should Avoid

- Chasing every competitor feature release.
- Maximizing the number of simultaneous agents.
- Loading all memories or skills into every prompt.
- Treating a larger context window as a substitute for context engineering.
- Using an LLM for deterministic repository analysis.
- Adding more providers before profiles, memory, context efficiency, and evaluation are useful.
- Optimizing token count while silently reducing task success.
- Allowing memories and skills to grow without evaluation and consolidation.
- Building a full IDE when terminal and browser clients already cover complementary workflows.
- Tightly coupling product intelligence to one model provider or agent runtime.

## Key Risks and Mitigations

### Memory becomes noisy or stale

Mitigation:

- Typed memory.
- Revision tracking.
- Dependency-based invalidation.
- Consolidation.
- Retrieval evaluation.

### Context paging removes information the model later needs

Mitigation:

- Full-fidelity backing store.
- Page-fault recovery.
- Fault-driven pinning.
- Replay tests.
- Minimum-resident task state.

### Aggressive cost limits create downstream rework

Mitigation:

- Preserve a minimum working set.
- Deterministic validation.
- Expand context when the agent repeatedly faults or retries.
- Compare total task cost, not the price of an individual call.
- Keep balanced and maximum-quality modes available.

### Self-generated skills make performance worse

Mitigation:

- Version every skill.
- Evaluate on held-out tasks.
- Promote only measurable improvements.
- Keep automatic rollback.

### The project graph becomes expensive to maintain

Mitigation:

- Incremental indexing.
- Content hashes.
- File watchers.
- Language-specific adapters.
- Start with LSP and git data already available to Aux.

### The roadmap becomes too broad

Mitigation:

- Build one vertical slice at a time.
- Require measurable exit criteria for each phase.
- Avoid ecosystem expansion before the core compounding loop works.

### The visual experience becomes decorative or misleading

Mitigation:

- Back task, activity, context, validation, change, and cost views with shared runtime projections.
- Make active work and outcomes more prominent than aggregate telemetry.
- Keep mechanical detail collapsed but available.
- Use explicit stale, disconnected, unverified, failed, and blocked states.
- Require visual regression, responsive-layout, accessibility, and task-comprehension checks.

## The Long-Term Vision

After sustained use, an Aux project should possess a durable intelligence layer independent of any single session or model.

The developer should be able to enter a repository and type:

```text
aux
> Continue the rate-limit work and update the frontend client.
```

Aux should already know:

- The current branch and unfinished task.
- The relevant backend and frontend repositories.
- The earlier architectural decision.
- The applicable skills.
- The source and generated client relationships.
- The minimum relevant context.
- The active Cost Governor mode and remaining task budget.
- The tests required to validate the work.

It should complete the task for fewer tokens than the first similar task, record what it learned, and improve the corresponding skills and cost policies only when replay shows a measurable benefit. Its warm technical workbench should make the active work, changes, validation, context, and cost obvious without forcing the developer to read raw event logs.

That is the product Aux should become:

> **A persistent, self-optimizing project brain that makes coding agents more efficient, more capable, and less expensive.**
