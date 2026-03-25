# IP/Prefix Lookup Page Feature

## TL;DR

> **Quick Summary**: Add an IP/Prefix lookup page to the RTBHID dashboard that allows users to search for IPs/prefixes and see which sources/exclusions they belong to. Results display hierarchically (most specific first) with full details.

> **Deliverables**:
> - Test infrastructure setup (vitest for frontend, Go testing patterns for backend)
> - Backend: `POST /api/v1/lookup` endpoint with single/bulk support
> - Frontend: `/web/src/pages/Lookup.tsx` page with search form and results table
> - Navigation integration in Sidebar.tsx

> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Test Infrastructure → Backend Store → Backend API → Frontend Page → Navigation

---

## Context

### Original Request
Add a lookup IP page so users can find which source/exclusion a specific IP/prefix belongs to. Must support both IPv4 and IPv6, single IPs, network prefixes, and bulk lookups.

### Interview Summary
- **Results Display**: Full details - source name, exclusion name, matched prefix, note, date added
- **IP Support**: Both IPv4 and IPv6
- **Match Type**: All matching prefixes hierarchically (most specific first)
- **Search Scope**: Both sources AND exclusions
- **Bulk Input**: Support multiple IPs/prefixes (one per line)
- **UI Layout**: Simple search box + results table
- **Testing**: Set up vitest as first test suite for the project

### Metis Review
**Critical Findings**:
- This is the FIRST test suite for the project - need to establish infrastructure
- Performance concern: Use direct SQL queries for lookup instead of loading all into memory
- Search semantics: Show ALL containing prefixes, ordered by specificity
- Edge cases: IPv4-mapped IPv6, host bits set, overlapping sources

---

## Work Objectives

### Core Objective
Add IP/Prefix lookup functionality that searches both sources and exclusions, returns hierarchical results, and supports bulk input.

### Concrete Deliverables
1. **Test Infrastructure**: Vitest configuration for frontend
2. **Backend Store Method**: `LookupIP()` in `store.go` with tests
3. **Backend API Endpoint**: `POST /api/v1/lookup`
4. **Frontend API Client**: `lookup()` method in `api.ts`
5. **Frontend Page**: `Lookup.tsx` with search form and results table
6. **Navigation**: Route in `App.tsx`, nav item in `Sidebar.tsx`

### Definition of Done
- [ ] Single IPv4 address lookup works
- [ ] Single IPv6 address lookup works
- [ ] CIDR prefix lookup works
- [ ] Bulk lookup works (up to 10 items)
- [ ] Results ordered by prefix specificity
- [ ] Page accessible from sidebar
- [ ] All tests pass

### Must Have
- Search form with IP/CIDR input
- Results table with: matched prefix, source/exclusion name, type, note, date
- Backend validation and error handling
- Test files for new code

### Must NOT Have
- Database schema changes
- Modifications to existing pages beyond navigation
- Tests for existing code
- Caching layer
- Export/download functionality

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO (first test setup)
- **Automated tests**: TDD approach
- **Framework**: Vitest (frontend), Go testing (backend)

### QA Policy
Every task MUST include agent-executed QA scenarios.

### Orkes Kanban Status Updates (MANDATORY)
Every task MUST update Orkes Kanban Board status at key milestones:
- **Start of task**: Update status to `IN_PROGRESS`
- **Task complete**: Update status to `DONE`
- **Task failed/needs review**: Update status to `IN_REVIEW`

Use `orkes_update_task_status` tool with:
- `taskId`: The task UUID from Orkes
- `status`: One of `TODO`, `IN_PROGRESS`, `IN_REVIEW`, `DONE`, `FAILED`

**Task ID Mapping:**
| Task | Ticket | Task ID |
|------|--------|---------|
| Task1 | RTBH-0001 | 315fcf93-f98b-4e0e-b4b8-1f701f9a42e4 |
| Task 2 | RTBH-0002 | fe820ae2-44fa-4488-b910-c8979379d6f4 |
| Task 3 | RTBH-0003 | 5fc8be22-4e5d-4b84-8dde-d70e4cd9e3ae |
| Task 4 | RTBH-0004 | f55960d8-cb3a-4029-9950-a7ffc80da061 |
| Task 5 | RTBH-0005 | 06a8bdf5-8b29-4fc5-ad7d-6406193753e0 |
| Task 6 | RTBH-0006 | 22bdb6b7-93ba-4287-a450-ec1dbeb85a82 |
| Task 7 | RTBH-0007 | 913f50a7-23ea-4cd4-9df5-aa6a7ae9eaa9 |
| Task 8 | RTBH-0008 | 317f97fb-7ddf-4ebb-a5e2-025287d0551e |
| Task 9 | RTBH-0009 | 47c8dd34-0423-431b-9dc6-c29d9dd26cda |
| Task 10 | RTBH-0010 | e6c7bb55-0dc0-41ed-8a27-eaeeaf2cd0a5 |
| Task11| RTBH-0011 | 05a276a0-ac11-4139-8242-0ad7ecccdb35 |
| Task12| RTBH-0012 | 4178a8d2-9630-4256-b489-678d2dfc529c |
| Task13| RTBH-0013 | 6194178a-8318-4549-9e56-02a36509ebf1 |
| Task14| RTBH-0014 | 9bdfe004-5fdb-4714-899c-dd4df390ef2e |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — infrastructure + foundation):
├── Task 1: Setup vitest for frontend testing [quick]
├── Task 2: Add TypeScript types in api.ts [quick]
├── Task 3: Add Go types in types.go [quick]
├── Task 4: Add LookupIP stub in store.go [quick]
└── Task 5: Setup backend test infrastructure [quick]

Wave 2 (After Wave 1 — core implementation):
├── Task 6: Write backend unit tests (RED) [unspecified-high]
├── Task 7: Implement LookupIP method (GREEN) [deep]
├── Task 8: Write API tests (RED) [unspecified-high]
├── Task 9: Implement API endpoint (GREEN) [deep]
└── Task 10: Implement frontend API client [quick]

Wave 3 (After Wave 2 — UI integration):
├── Task 11: Write frontend tests (RED) [unspecified-high]
├── Task 12: Implement Lookup page component [visual-engineering]
├── Task 13: Add navigation integration [quick]
└── Task 14: Add frontend form validation [quick]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | — | 6, 11 |
| 2 | — | 10 |
| 3 | — | 6, 8 |
| 4 | — | 6 |
| 5 | — | 6, 8 |
| 6 | 1, 3, 4, 5 | 7 |
| 7 | 6 | — |
| 8 | 3, 5 | 9 |
| 9 | 8 | — |
| 10 | 2 | 11 |
| 11 | 1, 10 | 12 |
| 12 | 11 | 13, 14 |
| 13 | 12 | — |
| 14 | 12 | — |

---

## TODOs

- [x] 1. Setup vitest for frontend testing (DONE)

  **What to do**:
  - Install vitest and @vitest/ui as dev dependencies
  - Create `web/vitest.config.ts` with React Testing Library config
  - Add `"test": "vitest"` script to package.json
  - Create `web/src/test/setup.ts` for testing utilities

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 6, Task 11
  - **Blocked By**: None

  **References**:
  - `web/package.json` - Add test scripts
  - `web/vite.config.ts` - Extend for testing

  **Acceptance Criteria**:
  - [ ] `web/vitest.config.ts` exists
  - [ ] `web/package.json` has `"test": "vitest"` script
  - [ ] `npm test` runs without errors

  **Commit**: YES
  - Message: `feat(tests): setup vitest for frontend testing`
  - Files: `web/vitest.config.ts`, `web/package.json`, `web/src/test/setup.ts`

---

- [x] 2. Add TypeScript types in api.ts

  **What to do**:
  - Add `LookupRequest`, `LookupMatch`, `LookupResult`, `LookupResponse` interfaces
  - Export all new types

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 10
  - **Blocked By**: None

  **References**:
  - `web/src/lib/api.ts:151-154` - Exclude interface pattern
  - `web/src/lib/api.ts:116-127` - Source interface pattern

  **Acceptance Criteria**:
  - [ ] All 4 interfaces defined with proper types
  - [ ] No TypeScript errors

  **Commit**: NO (groups with Task 10)

---

- [x] 3. Add Go types in types.go

  **What to do**:
  - Add `LookupRequest`, `LookupMatch`, `LookupResult`, `LookupResponse` structs
  - Add proper JSON tags for all fields

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 6, Task 8
  - **Blocked By**: None

  **References**:
  - `internal/portal/state/types.go` - Existing struct patterns

  **Acceptance Criteria**:
  - [ ] All 4 structs defined with JSON tags
  - [ ] `go build ./...` succeeds

  **Commit**: NO (groups with Task 7)

---

- [x] 4. Add LookupIP stub in store.go

  **What to do**:
  - Add `LookupIP(queries []string) ([]LookupResult, error)` method signature
  - Add empty implementation that returns "not implemented" error

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 6
  - **Blocked By**: None

  **References**:
  - `internal/portal/state/store.go:50-100` - Existing Store interface

  **Acceptance Criteria**:
  - [ ] Method signature exists in Store interface
  - [ ] Build succeeds

  **Commit**: NO (groups with Task 7)

---

- [x] 5. Setup backend test infrastructure

  **What to do**:
  - Create `internal/portal/state/store_lookup_test.go`
  - Add test helper functions

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 6
  - **Blocked By**: None

  **Acceptance Criteria**:
  - [ ] Test file exists with package declaration
  - [ ] `go test ./internal/portal/state/... -v` runs

  **Commit**: NO (groups with Task 6)

---

- [x] 6. Write backend unit tests (RED)

  **What to do**:
  - Write table-driven tests for LookupIP covering:
    - Single IPv4 lookup
    - Single IPv6 lookup
    - CIDR prefix lookup
    - Bulk lookup
    - Invalid input
    - Empty result
    - Hierarchical ordering
  - All tests should FAIL initially

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T1, T3, T4, T5

  **References**:
  - `internal/portal/state/store.go:592-626` - normalizeCIDR function
  - `internal/portal/state/store.go:507-586` - EffectivePrefixes pattern

  **Acceptance Criteria**:
  - [ ] Tests for 8+ scenarios defined
  - [ ] All tests FAIL (RED phase)

  **Commit**: YES
  - Message: `test(lookup): add LookupIP unit tests (RED)`
  - Files: `internal/portal/state/store_lookup_test.go`

---

- [x] 7. Implement LookupIP method (GREEN)

  **What to do**:
  - Implement `LookupIP(queries []string) ([]LookupResult, error)`
  - Parse and normalize CIDR using `normalizeCIDR()`
  - Match against sources and excludes using `net/netip.Prefix.Contains()`
  - Order results by specificity (longest prefix first)
  - Handle IPv4-mapped IPv6 addresses

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T6

  **References**:
  - `internal/portal/state/store.go:592-613` - normalizeCIDR
  - `internal/portal/state/store.go:615-625` - isExcludedByCIDR
  - `internal/portal/state/store.go:507-586` - EffectivePrefixes

  **Acceptance Criteria**:
  - [ ] All tests from Task 6 pass (GREEN)
  - [ ] Results ordered correctly

  **Commit**: YES
  - Message: `feat(lookup): implement LookupIP method`
  - Files: `internal/portal/state/store.go`, `internal/portal/state/types.go`

---

- [x] 8. Write API tests (RED)

  **What to do**:
  - Create `internal/portal/httpapi/server_lookup_test.go`
  - Write HTTP handler tests for `/api/v1/lookup`
  - Test: valid single IP, valid CIDR, bulk request, invalid input
  - Tests should FAIL initially

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T3, T5

  **Acceptance Criteria**:
  - [ ] Test file exists
  - [ ] All tests FAIL (RED)

  **Commit**: NO (groups with Task 9)

---

- [x] 9. Implement API endpoint (GREEN)

  **What to do**:
  - Add `handleLookup()` handler in `server.go`
  - Wire up route `POST /api/v1/lookup`
  - Parse JSON request body
  - Call `store.LookupIP()`
  - Return JSON response

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T8

  **References**:
  - `internal/portal/httpapi/server.go` - Existing handler patterns
  - `internal/portal/httpapi/server.go:1-100` - Route wiring

  **Acceptance Criteria**:
  - [ ] `POST /api/v1/lookup` endpoint works
  - [ ] All tests from Task 8 pass
  - [ ] Returns proper JSON response

  **Commit**: YES
  - Message: `feat(lookup): add POST /api/v1/lookup endpoint`
  - Files: `internal/portal/httpapi/server.go`

---

- [x] 10. Implement frontend API client

  **What to do**:
  - Add `lookup(queries: string[]): Promise<LookupResponse>` method to `api.ts`
  - Follow existing API method patterns
  - Handle errors with try/catch

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T2

  **References**:
  - `web/src/lib/api.ts:1-50` - Existing API methods

  **Acceptance Criteria**:
  - [ ] `lookup()` method exists in api object
  - [ ] TypeScript types match backend
  - [ ] No TypeScript errors

  **Commit**: YES
  - Message: `feat(lookup): add frontend API client`
  - Files: `web/src/lib/api.ts`

---

- [x] 11. Write frontend tests (RED)

  **What to do**:
  - Create `web/src/pages/Lookup.test.tsx`
  - Write tests for:
    - Renders search form
    - Handles valid IP input
    - Handles invalid input
    - Displays results correctly
    - Handles empty results
  - Tests should FAIL initially

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T1, T10

  **Acceptance Criteria**:
  - [ ] Test file exists
  - [ ] All tests FAIL (RED)

  **Commit**: NO (groups with Task 12)

---

- [x] 12. Implement Lookup page component

  **What to do**:
  - Create `web/src/pages/Lookup.tsx`
  - Follow `Prefixes.tsx` pattern
  - Add search textarea for IP/CIDR input (bulk support)
  - Add "Search" button
  - Add results table with columns: Query, Type, Source/Exclusion, CIDR, Note, Date
  - Handle loading, error, and empty states
  - Call `api.lookup()` on form submit

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**:[]

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T11

  **References**:
  - `web/src/pages/Prefixes.tsx` - Page structure pattern
  - `web/src/pages/Sources.tsx` - Results table pattern
  - `web/src/lib/api.ts` - API client from Task 10

  **Acceptance Criteria**:
  - [ ] Page renders with search form
  - [ ] Results table displays correctly
  - [ ] Handles single and bulk input
  - [ ] Shows loading/error states
  - [ ] All tests from Task 11 pass

  **Commit**: YES
  - Message: `feat(lookup): add Lookup page component`
  - Files: `web/src/pages/Lookup.tsx`, `web/src/pages/Lookup.test.tsx`

---

- [x] 13. Add navigation integration

  **What to do**:
  - Add route in `web/src/App.tsx`: `<Route path="/lookup" element={<Lookup />} />`
  - Add nav item in `web/src/components/layout/Sidebar.tsx`
  - Use `Search` icon from lucide-react

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T12

  **References**:
  - `web/src/App.tsx` - Existing routes
  - `web/src/components/layout/Sidebar.tsx` - navItems array

  **Acceptance Criteria**:
  - [ ] `/lookup` route works
  - [ ] Nav item appears in sidebar
  - [ ] Page accessible from navigation

  **Commit**: YES
  - Message: `feat(lookup): add navigation to Lookup page`
  - Files: `web/src/App.tsx`, `web/src/components/layout/Sidebar.tsx`

---

- [x] 14. Add frontend form validation

  **What to do**:
  - Add client-side validation using `ipaddr.js`
  - Validate each line is valid IP or CIDR
  - Show error message for invalid input
  - Disable submit button if input invalid

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: T12

  **References**:
  - Install: `cd web && npm install ipaddr.js`
  - Usage: `ipaddr.isValidCIDR()` method

  **Acceptance Criteria**:
  - [ ] Invalid IP shows error message
  - [ ] Valid IP allows submit
  - [ ] Install `ipaddr.js` dependency

  **Commit**: YES
  - Message: `feat(lookup): add frontend form validation`
  - Files: `web/src/pages/Lookup.tsx`, `web/package.json`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Verify all "Must Have" items are implemented. Check evidence files exist. Compare deliverables against plan.

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + lint + tests. Check for: `as any`, empty catches, console.log, unused imports.

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Test: single IPv4, single IPv6, CIDRprefix, bulk input, invalid input. Verify results display correctly.

- [ ] F4. **Scope Fidelity Check** — `deep`
  Verify no database schema changes, no modifications to existing pages beyond navigation, no caching layer.

---

## Commit Strategy

Atomic commits for each major task:
1. `feat(tests): setup vitest for frontend testing`
2. `test(lookup): add LookupIP unit tests (RED)`
3. `feat(lookup): implement LookupIP method`
4. `feat(lookup): add POST /api/v1/lookup endpoint`
5. `feat(lookup): add frontend API client`
6. `feat(lookup): add Lookup page component`
7. `feat(lookup): add navigation to Lookup page`
8. `feat(lookup): add frontend form validation`

---

## Success Criteria

### Verification Commands
```bash
# Backend tests
go test ./internal/portal/state/... -v

# Frontend tests
cd web && npm test

# TypeScript check
cd web && npx tsc --noEmit

# Lint
cd web && npm run lint

# Build
make portal
```

### Final Checklist
- [ ] Single IPv4 lookup works
- [ ] Single IPv6 lookup works
- [ ] CIDR prefix lookup works
- [ ] Bulk lookup works (up to 10 items)
- [ ] Results ordered by specificity
- [ ] Invalid input shows error
- [ ] Page accessible from sidebar
- [ ] All tests pass