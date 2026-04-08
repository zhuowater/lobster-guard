# Lobster Guard P1 Refactor Plan — Config Persistence and LLM Pipeline

> Scope: formal P1 follow-up after fixing engine-toggle YAML writeback and UI state drift.

## Context

Recent fixes addressed two P0 classes of bugs:

1. Engine toggles not being written back correctly to `config.yaml`
2. Settings UI showing stale or incorrect engine state after refresh

Those are now stabilized by:
- correct YAML key persistence in `src/api_config.go`
- flat `engine_toggles` contract in `GET /api/v1/config/settings`
- regression tests in `src/config_settings_test.go`

The remaining work is structural: reduce future config drift, reduce duplicated read-modify-write logic, and make the LLM enforcement path easier to reason about and test.

---

## Goals

### Goal 1 — Unify config persistence
Make all config writes use one shared persistence service instead of ad hoc read-modify-write logic in multiple API files.

### Goal 2 — Stabilize config/API contracts
Stop frontend/backend drift by introducing explicit DTOs for config responses instead of leaking raw Go struct JSON shapes.

### Goal 3 — Decompose `llm_proxy.go`
Split the LLM proxy into explicit phases so request, response, and streaming behavior can be tested independently.

### Goal 4 — Improve regression coverage
Add tests around config persistence, config contract shape, conf.d sync behavior, and LLM pipeline ordering.

---

## Non-goals

- No feature expansion
- No dashboard redesign
- No behavior changes to policy semantics unless required to preserve current behavior during refactor
- No protocol redesign for Gateway/WSS in this phase

---

# Workstream A — Config Persistence Service

## Problem
Config writes are currently distributed across multiple files:
- `src/api_config.go`
- `src/api_llm.go`
- `src/api_route.go`
- `src/api_rules.go`
- `src/api_v32.go`
- potentially other API helpers over time

This creates risk of:
- some endpoints updating memory but not disk
- some endpoints updating `config.yaml` but not `conf.d`
- section merge behavior being inconsistent
- future writeback bugs reappearing in new APIs

## Deliverable
Create a single shared config persistence component.

## Proposed files
- Create: `src/config_persistence.go`
- Create: `src/config_persistence_test.go`

## Proposed API
```go
type ConfigPersistence struct {
    mu      *sync.Mutex
    cfgPath string
}

func NewConfigPersistence(mu *sync.Mutex, cfgPath string) *ConfigPersistence
func (p *ConfigPersistence) LoadRaw() (map[string]interface{}, error)
func (p *ConfigPersistence) SaveRaw(raw map[string]interface{}) error
func (p *ConfigPersistence) PatchTopLevel(key string, value interface{}) error
func (p *ConfigPersistence) PatchSection(section string, patch map[string]interface{}) error
func (p *ConfigPersistence) PatchWith(fn func(raw map[string]interface{}) error) error
func (p *ConfigPersistence) SyncConfDSection(section string, value interface{}) error
```

## Requirements
- One lock path only: reuse existing `cfgMu`
- Preserve unrelated YAML fields
- Handle both `map[string]interface{}` and `map[interface{}]interface{}` safely
- Support optional `conf.d` section sync for sections like `llm_proxy`
- Return explicit errors with section/key context

## Migration targets
Migrate these callers to the service:
1. `handleConfigSettingsUpdate` in `src/api_config.go`
2. `handleAlertsConfigUpdate` in `src/api_config.go`
3. `saveLLMConfig` in `src/api_llm.go`
4. `saveRoutePolicies` in `src/api_route.go`
5. `persistOutboundRules` in `src/api_rules.go`
6. `persistRawSection` in `src/api_v32.go`

## Verification
- Existing tests pass
- New persistence tests cover:
  - top-level bool patch
  - nested section patch
  - map type compatibility
  - concurrent callers serialized by shared mutex
  - `conf.d` sync for `llm_proxy`

---

# Workstream B — Explicit Config DTOs / Contracts

## Problem
The UI previously depended on raw Go JSON serialization shape:
- PascalCase for some fields
- nested struct shape for others
- missing `json` tags created drift risk

The new `engine_toggles` field fixes this for one area, but the broader issue remains.

## Deliverable
Introduce explicit response DTOs for management config APIs instead of exposing the entire raw `Config` struct as the frontend contract.

## Proposed files
- Create: `src/config_dto.go`
- Create: `src/config_dto_test.go`

## Phase 1 DTO targets
### `GET /api/v1/config/settings`
Introduce a typed response with stable keys:
- `basic`
- `security`
- `rate_limit`
- `session`
- `alerts`
- `advanced`
- `engine_toggles`

Example:
```json
{
  "basic": {
    "inbound_listen": ":8443",
    "outbound_listen": ":8444",
    "management_listen": ":9090"
  },
  "security": {
    "inbound_detect_enabled": true,
    "outbound_audit_enabled": true,
    "detect_timeout_ms": 200
  },
  "engine_toggles": {
    "engine_inbound_detect": true,
    "engine_session_detect": false
  }
}
```

## Migration approach
- Keep old fields temporarily for compatibility
- Make frontend consume DTO fields first
- Remove raw-struct assumptions later once UI is migrated and verified

## Verification
- add tests to validate presence and type of DTO fields
- add snapshot-like contract tests for `engine_toggles` and basic config sections

---

# Workstream C — `Settings.vue` Decomposition

## Problem
`dashboard/src/views/Settings.vue` is too large and mixes unrelated concerns:
- config form
- engine toggles
- auth/token
- backup
- sqlite stats
- system health
- LLM config
- demo data

This increases the probability of future frontend contract drift.

## Deliverable
Split the page into focused tab components.

## Proposed files
- Create: `dashboard/src/views/settings/SettingsConfigTab.vue`
- Create: `dashboard/src/views/settings/SettingsEngineTab.vue`
- Create: `dashboard/src/views/settings/SettingsLLMTab.vue`
- Create: `dashboard/src/views/settings/SettingsSystemTab.vue`
- Create: `dashboard/src/views/settings/SettingsDatabaseTab.vue`
- Modify: `dashboard/src/views/Settings.vue`

## Rules
- No UI redesign in this phase
- Preserve existing behavior and copy
- Extract logic with minimal renaming first
- Move API calls closest to the tab that uses them unless shared state requires a small composable

## Optional follow-up
If useful after extraction:
- `dashboard/src/composables/useConfigSettings.js`
- `dashboard/src/composables/useEngineToggles.js`

---

# Workstream D — LLM Proxy Phase Decomposition

## Problem
`src/llm_proxy.go` currently bundles:
- transport proxying
- tenant resolution
- request policy checks
- response policy checks
- stream/SSE handling
- taint reversal
- rewrite logic
- auditing and metrics

This makes ordering fragile and debugging expensive.

## Deliverable
Split the LLM proxy into explicit pipeline stages.

## Proposed files
- Create: `src/llm_request_preprocess.go`
- Create: `src/llm_request_policy.go`
- Create: `src/llm_upstream_forward.go`
- Create: `src/llm_response_policy.go`
- Create: `src/llm_stream_sse.go`
- Create: `src/llm_observability.go`
- Shrink: `src/llm_proxy.go` into orchestration only

## Target phase order
```text
1. Resolve tenant / request context
2. Parse request + request preprocessing
3. Request policy checks
4. Forward upstream
5. Response transform / rewrite
6. Response policy checks
7. SSE / streaming finalization
8. Audit / metrics / trace finalization
```

## Constraints
- No semantic reordering unless explicitly intended and tested
- Preserve current streaming behavior exactly during extraction
- Every extracted stage must have unit-testable helpers

## Required tests
- request-side taint reversal test
- response-side rewrite test
- SSE tail rewrite test
- reasoning_content / content fallback test
- tool policy enforcement ordering test
- counterfactual invocation gating test

---

# Workstream E — Config/Conf.d Precedence Rules

## Problem
The codebase already has partial `conf.d` synchronization, but the precedence model is not explicit enough.

## Deliverable
Document and enforce config precedence rules.

## Proposed artifact
- Create: `docs/config-precedence.md`

## Must define
- what lives only in main `config.yaml`
- what may be overridden by `conf.d`
- when API writes back to `config.yaml`
- when API must also sync `conf.d`
- what happens on restart if both contain the same section

## Optional code follow-up
Add helper in persistence layer:
```go
func (p *ConfigPersistence) UpdateSectionWithConfDSync(section string, value interface{}) error
```

---

# Testing Plan

## New tests to add
### Backend
- `src/config_persistence_test.go`
  - patch top-level key
  - patch nested section
  - preserve unrelated fields
  - conf.d sync for `llm_proxy`
  - concurrent write serialization

- `src/config_dto_test.go`
  - stable response shape for `/api/v1/config/settings`
  - all `engine_toggles` present and typed as bool

- `src/llm_proxy_pipeline_test.go`
  - request/response/streaming stage ordering

### Frontend
If frontend testing is adopted in this phase:
- `dashboard/src/views/settings/__tests__/SettingsEngineTab.test.js`
  - reads `engine_toggles`
  - toggle UI state reflects backend data
- `dashboard/src/views/settings/__tests__/SettingsConfigTab.test.js`
  - form save issues correct payload shape

## Mandatory regression commands
```bash
cd /root/lobster-guard/dashboard && npm run build
cd /root/lobster-guard/src && go test ./...
cd /root/lobster-guard/src && go build ./...
```

---

# Suggested implementation order

## Phase P1.1 — Persistence foundation
1. add `config_persistence.go`
2. migrate `handleConfigSettingsUpdate`
3. migrate `saveLLMConfig`
4. add persistence tests

## Phase P1.2 — Stable config DTO
5. add `engine_toggles`-style DTO expansion for config sections
6. migrate Settings UI to DTO-first reads
7. add DTO contract tests

## Phase P1.3 — Settings page extraction
8. split `Settings.vue` into tab components
9. keep behavior identical
10. run frontend build + full Go regression suite

## Phase P1.4 — LLM proxy extraction
11. extract request preprocess stage
12. extract response policy stage
13. extract SSE stage
14. add ordering and streaming regressions

## Phase P1.5 — Config precedence hardening
15. document `config.yaml` / `conf.d` precedence
16. unify section sync policy in persistence layer

---

# Risks

## Risk 1 — Silent contract breakage
Mitigation:
- DTO tests
- keep compatibility fields temporarily

## Risk 2 — LLM pipeline reorder bug
Mitigation:
- stage-by-stage extraction
- snapshot-like behavior tests around streaming and rewrite semantics

## Risk 3 — Config persistence migration regression
Mitigation:
- migrate one caller at a time
- verify unchanged YAML sections in tests

---

# Success criteria

P1 is complete when:
- config writes no longer use ad hoc read-modify-write logic in API handlers
- `/api/v1/config/settings` has explicit stable DTO/contract fields
- `Settings.vue` is decomposed enough that config, engines, and LLM tabs are isolated
- `llm_proxy.go` becomes orchestration instead of a monolithic implementation file
- config persistence, DTO contract, and LLM stage ordering all have regression tests

---

# Immediate next recommended action

Implement **Workstream A** first:
1. create `src/config_persistence.go`
2. migrate `handleConfigSettingsUpdate`
3. migrate `saveLLMConfig`
4. add tests before moving on to UI decomposition
