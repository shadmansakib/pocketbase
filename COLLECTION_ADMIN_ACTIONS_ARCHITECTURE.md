# Collection Admin Actions Architecture

This document is the single source of architecture for adding Django-style admin actions to PocketBase collection records.

It is based on the current PocketBase codebase and follows the repo conventions documented in `AGENTS.md`.

## 1. Objective

Replace the current records-page bottom bulk-action UI with a Django-admin-style actions bar that:

- lives near the records search controls
- uses a dropdown + `Go` workflow
- applies only to selected records
- keeps the current built-in `Delete` and `Export JSON` implementations intact
- supports custom actions registered from Go
- supports custom actions registered from PocketBase JS hooks
- supports both synchronous and asynchronous custom actions in the initial implementation

## 2. Locked Product Decisions

The following product decisions are now fixed for this feature:

- Actions operate on selected records only.
- Selection semantics stay the same as the current records list behavior:
  - client-side
  - currently loaded records only
  - no “select all matching filter across all pages”
- The initial release does not support extra action input or Django-style intermediate pages.
- The existing built-in `Delete` and `Export JSON` logic stays client-side and keeps its current behavior.
- Only the UI changes for the built-ins:
  - move from the records-page bottom bulkbar/popup affordance
  - into a top actions dropdown + `Go` button
- Custom actions must support both sync and async execution from the start.
- This is a superuser dashboard feature, not a public client API feature.

## 3. Current State Analysis

### 3.1 Current records page composition

Today the collection records page is composed primarily by:

- `ui/src/collections/pageCollections.js`
- `ui/src/records/recordsSearchbar.js`
- `ui/src/records/recordsList.js`

Current structure:

```text
pageCollections
  -> recordsSearchbar
  -> recordsList
       -> table
       -> row selection
       -> bottom bulkbar
```

### 3.2 Current selection and built-in action implementation

Bulk selection is currently owned by `ui/src/records/recordsList.js`:

- `data.bulkSelected`
- `selectAll()`
- `downloadSelected()`
- `deleteSelected()`

Current built-ins are fully frontend-driven:

- `Delete`
  - confirms in the UI
  - batches deletes in groups of 100
  - calls `app.pb.collection(...).delete(id)`
- `Export JSON`
  - uses the already loaded selected records
  - removes `expand`
  - downloads via `app.utils.downloadJSON(...)`

The user explicitly wants these implementations preserved.

### 3.3 Existing extension points relevant to this feature

Relevant backend and extension patterns already exist:

- typed request events in `core/events.go`
- route binding patterns in `apis/*`
- experimental UI extensions through `ServeEvent.UIExtensions`
- JS helper-style bindings in `plugins/jsvm/binds.go`
- admin UI mount/unmount `pbEvent` anchors in `ui/src/main.js`

### 3.4 Important implementation constraints from the current codebase

- `recordsList.js` currently owns selection state, but the new top actions bar also needs that state.
- `pageCollections.js` already owns filter, sort, active collection, and reset state, so selection should move there too.
- JS VM handlers are not Promise-first today. Async collection actions must therefore mean durable background execution on the Go side, not Promise-based JS handlers.
- `ui/src/css/bulkbar.css` is also used by logs UI. The records-page bulkbar can be removed without deleting the shared CSS file or disturbing logs.
- PocketBase currently has examples of fire-and-forget background work, but not a first-class durable admin jobs subsystem. This feature needs to add one for async actions to be trustworthy.

## 4. Scope Boundaries

### 4.1 In scope

- Records-page actions bar under the search controls
- Built-in `Delete selected records`
- Built-in `Export selected records as JSON`
- Go-registered custom actions
- JS-hook-registered custom actions
- Sync custom action execution
- Durable async custom action execution
- Admin-visible async job status APIs
- Minimal async job visibility in the dashboard UI

### 4.2 Out of scope

- Cross-page “select all matching current filter”
- Extra action input forms
- Django-style intermediate pages
- Reworking the logs bulkbar in the same feature
- Exposing this feature to non-superuser clients
- Automatic retry policies for interrupted async jobs
- Cancel/resume controls for async jobs

## 5. Recommended UX

### 5.1 Placement

Recommended placement:

- directly below `recordsSearchbar`
- directly above `recordsList`

This keeps the actions in the same visual zone as search, filtering, and selection.

Recommended page structure:

```text
pageCollections
  -> recordsSearchbar
  -> recordsActionsBar
  -> recordsList
```

### 5.2 Controls

The actions bar should contain:

- selection summary
- action dropdown
- `Go` button
- `Reset` button

Recommended wireframe:

```text
+------------------------------------------------------------------+
| [ Search / filter controls                                   ]   |
+------------------------------------------------------------------+
| 3 selected   [ Action: Delete selected records v ] [Go] [Reset]  |
+------------------------------------------------------------------+
| [x] | title | status | updated | ...                             |
| [x] | ...                                                        |
| [ ] | ...                                                        |
+------------------------------------------------------------------+
```

### 5.3 Interaction rules

- The bar is visible whenever a collection is active.
- The dropdown is visible even when nothing is selected.
- `Go` is disabled until:
  - at least one record is selected
  - an action is selected
  - the action’s min/max rules are satisfied
- `Reset` clears the current selection immediately.
- `Delete selected records` keeps the current confirm modal and delete batching behavior.
- `Export selected records as JSON` keeps the current download behavior and naming.
- After collection, filter, sort, or manual list reset changes, selection is cleared.
- If a selected record is deleted while the page is open, its id must be pruned from selection state.

### 5.4 Built-in action labels

Recommended built-in labels:

- `Delete selected records`
- `Export selected records as JSON`

`Delete selected records` should remain hidden for view collections, matching current behavior.

## 6. Architecture Overview

The cleanest design is to split the feature into three layers:

1. Frontend selection and actions-bar layer
2. Server-side action registry and sync execution layer
3. Durable async jobs layer

High-level flow:

```text
recordsList
  -> updates selected ids/records
pageCollections
  -> owns selection state
recordsActionsBar
  -> built-in client action
  -> or call records actions API

records actions API
  -> resolve collection
  -> resolve action from registry
  -> validate selection
  -> sync: run handler and return result
  -> async: enqueue durable job and return job summary

async worker
  -> loads queued job
  -> resolves action again
  -> runs handler
  -> stores progress/result/failure

dashboard UI
  -> polls recent active jobs
  -> shows status and terminal feedback
```

Async lifecycle:

```text
User clicks Go
  -> POST /records/actions/{action}
  -> job persisted as queued
  -> 202 Accepted + job summary
  -> frontend clears selection if configured
  -> frontend polls job status
  -> worker marks running
  -> handler completes
  -> worker marks succeeded or failed
  -> frontend shows terminal toast and refreshes list if configured
```

## 7. Frontend Architecture

### 7.1 Move selection ownership to `pageCollections`

Selection should move from `ui/src/records/recordsList.js` into `ui/src/collections/pageCollections.js`.

Recommended new page-owned state:

- `bulkSelected`
- `recentActionJobs`
- `activeActionPollers` or equivalent lightweight tracking

Why this matters:

- both `recordsList` and `recordsActionsBar` need the same selection state
- page-level resets already happen in `pageCollections`
- the parent page is the right place to clear selection on collection/filter/sort changes

Recommended shape:

```text
pageCollections
  owns:
    - activeCollection
    - filter
    - sort
    - reset
    - bulkSelected
    - recentActionJobs
```

### 7.2 Refactor `recordsList` into a controlled selection component

`recordsList` should keep:

- records fetching
- row rendering
- checkbox UI
- “select all currently loaded records” behavior

`recordsList` should stop owning:

- built-in action execution
- bottom bulkbar rendering
- authoritative selection state

Recommended props:

- `bulkSelected`
- `onbulkselectchange`

Recommended behavior:

- row checkbox changes emit a new selected map
- “select all” emits a new selected map
- record delete/save refreshes keep list behavior intact
- records-page `.bulkbar` rendering is removed

### 7.3 New component: `recordsActionsBar`

Add:

- `ui/src/records/recordsActionsBar.js`

Responsibilities:

- render selection summary
- render action dropdown + `Go` + `Reset`
- register built-in client actions
- load server-provided custom actions for the active collection
- merge and sort built-in + server actions
- enforce selection rules client-side before execution
- execute built-in client actions
- execute sync custom actions
- enqueue async custom actions
- poll async jobs
- show success/failure toasts

### 7.4 Built-in actions remain frontend-resident

The built-ins are not moved to the new server action API in the initial implementation.

Keep them exactly as frontend actions:

- `delete_selected`
- `export_json`

Preserve the current behavior:

- delete confirmation copy
- delete batching
- success toast semantics
- JSON export payload construction
- JSON filename behavior

This keeps risk low and avoids mixing a UI migration with a behavior rewrite.

### 7.5 Action source merging

The dropdown should merge:

1. frontend built-ins
2. server-provided custom actions

Recommended ordering:

- built-ins first
- then custom actions sorted by explicit `order`
- then by `label`

Important note:

- the list endpoint returns server actions only
- built-ins are always injected locally by the frontend

This ensures the bar still works even if the custom-actions endpoint fails.

### 7.6 Sync custom action UX

For sync custom actions:

- `Go` triggers a POST request
- the UI stays locally pending for that request
- on success:
  - show toast from response message or a default success message
  - clear selection if configured
  - refresh list if configured
- on failure:
  - surface the API error through existing error handling

### 7.7 Async custom action UX

For async custom actions:

- `Go` triggers a POST request
- the server returns `202 Accepted` with a job summary
- the UI immediately:
  - shows a queued toast
  - stores the job in `recentActionJobs`
  - clears selection if configured
  - starts polling the job

Recommended minimum async UI:

- a small recent-jobs section directly under the actions bar
- each item shows:
  - action label
  - status
  - progress when available
  - started/updated time

This keeps async status close to where the user launched the action and avoids requiring a separate admin page before the feature is usable.

Recommended polling behavior:

- poll active jobs only
- stop polling when a job reaches a terminal state
- on success:
  - toast success
  - refresh records if job metadata requires it
- on failure:
  - toast failure with a concise message

Recommended recent-jobs loading:

- when the active collection changes, load recent jobs for that collection
- include terminal jobs in a short recent history window
- keep the list small, for example the most recent 10 jobs

### 7.8 Suggested frontend extension anchors

Add new `pbEvent` anchors:

- `recordsActionsBar`
- `recordsActionsRecentJobs`

This leaves room for future UI extensions without forcing a rewrite.

## 8. Backend Architecture

### 8.1 First-class collection action registry

Add a runtime registry rather than package globals or ad hoc app store usage.

Recommended app API:

```text
app.CollectionActions()
  -> Add(...)
  -> Remove(...)
  -> List(...)
  -> Resolve(collection, actionName)
```

This matches PocketBase’s typed core style and makes both Go and JS registration predictable.

### 8.2 Core action types

Recommended new file:

- `core/collection_action.go`

Recommended shape:

```go
type CollectionActionExecutionMode string

const (
    CollectionActionExecutionSync  CollectionActionExecutionMode = "sync"
    CollectionActionExecutionAsync CollectionActionExecutionMode = "async"
)

type CollectionAction struct {
    Name               string
    Label              string
    Description        string
    Icon               string
    Order              int
    Collections        []string
    ExcludeCollections []string
    SelectionRequired  bool
    MinSelection       int
    MaxSelection       int
    ConfirmText        string
    Variant            string
    ExecutionMode      CollectionActionExecutionMode
    LoadRecords        bool
    ClearSelection     bool
    ReloadRecords      bool
    Handler            func(e *CollectionActionRequestEvent) error `json:"-"`
}

type CollectionActionResult struct {
    Message        string `json:"message,omitempty"`
    Data           any    `json:"data,omitempty"`
    ClearSelection bool   `json:"clearSelection,omitempty"`
    ReloadRecords  bool   `json:"reloadRecords,omitempty"`
    Job            any    `json:"job,omitempty"`
}
```

Important notes:

- `MaxSelection == 0` should mean “no upper limit”.
- `Handler` stays runtime-only.
- `ExecutionMode` is part of the public action contract from day one.

### 8.3 Request event shape

Recommended event type:

```go
type CollectionActionRequestEvent struct {
    hook.Event
    *RequestEvent
    baseCollectionEventData

    Action    *CollectionAction
    RecordIds []string
    Records   []*Record
    Payload   map[string]any
    Job       *CollectionActionJob
    Result    *CollectionActionResult
}
```

Recommended helper methods:

- `SetProgress(processed, total int, message string) error`
- `SetResultMessage(message string)`

Notes:

- `Job == nil` for sync actions
- `Job != nil` for async worker execution
- progress is optional and only meaningful for async jobs

### 8.4 Registry behavior

Recommended registry responsibilities:

- concurrent-safe storage
- replace-by-name semantics
- collection matching by id or name
- stable sort order
- metadata validation on registration

Collection matching rule:

- if `Collections` is empty, the action applies to all collections
- otherwise match by collection id or name
- `ExcludeCollections` removes specific collections after inclusion

### 8.5 API endpoints

Recommended route family:

- `GET /api/collections/{collection}/records/actions`
- `POST /api/collections/{collection}/records/actions/{action}`
- `GET /api/action-jobs`
- `GET /api/action-jobs/{id}`

All of these should require superuser authentication.

### 8.6 List endpoint behavior

`GET /api/collections/{collection}/records/actions`

Responsibilities:

- resolve the collection
- filter registry actions by applicability
- expose metadata only
- never expose handler internals

Important:

- this endpoint returns custom server actions only
- built-ins are still injected by the frontend

### 8.7 Execute endpoint behavior

`POST /api/collections/{collection}/records/actions/{action}`

Recommended request body:

```json
{
  "ids": ["record1", "record2"],
  "payload": {}
}
```

Validation rules:

- collection exists
- action exists and applies to that collection
- ids are unique
- selection satisfies min/max rules
- empty selection is rejected when selection is required

Execution behavior:

- if `ExecutionMode == "sync"`:
  - validate
  - optionally load records
  - run handler
  - return `200 OK`
- if `ExecutionMode == "async"`:
  - validate
  - persist job as `queued`
  - return `202 Accepted`
  - worker executes later

### 8.8 Durable async job subsystem

Async execution should not be a fire-and-forget goroutine. It should be a durable job system specifically for collection actions.

Recommended new files:

- `core/collection_action_job.go`
- `core/collection_action_job_query.go`
- `apis/action_jobs.go`

Recommended persistence:

- store jobs in `auxiliary.db`
- follow the existing aux-db pattern already used for `_logs`

Recommended migration:

- add a new auxiliary migration under `migrations/`
- create an `_action_jobs` table in `auxiliary.db`

Recommended persisted fields:

- `id`
- `actionName`
- `actionLabel`
- `collectionId`
- `collectionName`
- `status`
- `recordIds`
- `payload`
- `requestedById`
- `requestedByEmail`
- `processedItems`
- `totalItems`
- `statusMessage`
- `result`
- `error`
- `started`
- `finished`
- `created`
- `updated`

Recommended statuses:

- `queued`
- `running`
- `succeeded`
- `failed`

### 8.9 Async worker behavior

Recommended initial worker model:

- default global concurrency of `1`
- jobs are processed in creation order
- async is used to decouple user requests, not to maximize parallel writes

Recommended job lifecycle:

1. enqueue job
2. mark `queued`
3. worker claims job and marks `running`
4. optionally load current records by id
5. execute handler
6. persist progress updates when provided
7. mark `succeeded` or `failed`

Startup recovery rule:

- any job left in `running` after process restart is marked `failed`
- use a clear interruption message such as `job interrupted by process restart`

Do not auto-retry interrupted jobs in the initial implementation.

Reasoning:

- many admin actions may have side effects
- auto-retry is unsafe without explicit idempotency guarantees

### 8.10 Record loading semantics

If `LoadRecords == true`:

- sync actions load current records before handler execution
- async actions load current records when the worker starts, not when the request is accepted

Recommended failure rule:

- if any selected id no longer resolves to a record at execution time, fail the job before handler execution

This is the safest default and avoids silent partial execution.

### 8.11 Transaction guidance

The framework should not auto-wrap every action in a transaction.

Recommended rule:

- action handlers choose their own transaction strategy
- use `RunInTransaction(...)` when the action needs all-or-nothing semantics
- allow best-effort actions when the action author explicitly wants them

This matches current PocketBase style better than a hidden global transaction wrapper.

### 8.12 Job status APIs

`GET /api/action-jobs/{id}`

Recommended response fields:

- `id`
- `actionName`
- `actionLabel`
- `collectionId`
- `collectionName`
- `status`
- `processedItems`
- `totalItems`
- `statusMessage`
- `error`
- `started`
- `finished`
- `updated`

`GET /api/action-jobs`

Recommended query support:

- `collection`
- `status`
- `limit`

This endpoint gives the UI a simple way to reload recent jobs for the active collection when the page is revisited.

### 8.13 Go registration API

Recommended example:

```go
app.CollectionActions().Add(&core.CollectionAction{
    Name:              "publish",
    Label:             "Publish selected records",
    Collections:       []string{"articles"},
    SelectionRequired: true,
    MinSelection:      1,
    LoadRecords:       true,
    ExecutionMode:     core.CollectionActionExecutionAsync,
    ClearSelection:    true,
    ReloadRecords:     true,
    Handler: func(e *core.CollectionActionRequestEvent) error {
        return e.App.RunInTransaction(func(txApp core.App) error {
            total := len(e.Records)

            for i, record := range e.Records {
                record.Set("status", "published")

                if err := txApp.Save(record); err != nil {
                    return err
                }

                if err := e.SetProgress(i+1, total, "Publishing records"); err != nil {
                    return err
                }
            }

            e.Result = &core.CollectionActionResult{
                Message:        fmt.Sprintf("Published %d records.", total),
                ClearSelection: true,
                ReloadRecords:  true,
            }

            return nil
        })
    },
})
```

### 8.14 JS hook registration API

Add helper bindings in `plugins/jsvm/binds.go`:

- `collectionActionAdd(definition, handler)`
- `collectionActionRemove(name)`

Recommended example:

```js
collectionActionAdd({
    name: "publish",
    label: "Publish selected records",
    collections: ["articles"],
    selectionRequired: true,
    minSelection: 1,
    loadRecords: true,
    executionMode: "async",
    clearSelection: true,
    reloadRecords: true,
}, (e) => {
    return e.app.runInTransaction((txApp) => {
        const total = e.records.length

        for (let i = 0; i < total; i++) {
            const record = e.records[i]
            record.set("status", "published")
            txApp.save(record)
            e.setProgress(i + 1, total, "Publishing records")
        }

        e.result = {
            message: `Published ${total} records.`,
            clearSelection: true,
            reloadRecords: true,
        }
    })
})
```

Important clarification:

- the JS handler is still synchronous code
- `executionMode: "async"` means the Go backend runs it in a durable background job
- it does not mean Promise-based JS execution

## 9. API Contract Summary

### 9.1 List actions

`GET /api/collections/{collection}/records/actions`

Example response:

```json
[
  {
    "name": "publish",
    "label": "Publish selected records",
    "description": "Sets status to published",
    "selectionRequired": true,
    "minSelection": 1,
    "maxSelection": 0,
    "confirmText": "Publish the selected records?",
    "executionMode": "async",
    "clearSelection": true,
    "reloadRecords": true
  }
]
```

### 9.2 Execute sync action

`POST /api/collections/{collection}/records/actions/{action}`

Example `200 OK` response:

```json
{
  "message": "Action completed successfully.",
  "clearSelection": true,
  "reloadRecords": true
}
```

### 9.3 Execute async action

`POST /api/collections/{collection}/records/actions/{action}`

Example `202 Accepted` response:

```json
{
  "message": "Action queued successfully.",
  "clearSelection": true,
  "reloadRecords": false,
  "job": {
    "id": "job_123",
    "actionName": "publish",
    "actionLabel": "Publish selected records",
    "collectionId": "articles_id",
    "collectionName": "articles",
    "status": "queued",
    "processedItems": 0,
    "totalItems": 25,
    "updated": "2026-04-21 12:00:00.000Z"
  }
}
```

### 9.4 Get job status

`GET /api/action-jobs/{id}`

Example response:

```json
{
  "id": "job_123",
  "actionName": "publish",
  "actionLabel": "Publish selected records",
  "collectionId": "articles_id",
  "collectionName": "articles",
  "status": "running",
  "processedItems": 12,
  "totalItems": 25,
  "statusMessage": "Publishing records",
  "error": "",
  "started": "2026-04-21 12:00:01.000Z",
  "finished": "",
  "updated": "2026-04-21 12:00:05.000Z"
}
```

## 10. Testing Strategy

### 10.1 Backend automated tests

Add tests for:

- actions list requires superuser auth
- actions list filters by collection applicability
- built-ins are not part of the server list response
- sync execute rejects unknown action
- sync execute rejects invalid selection payload
- sync execute loads records when `LoadRecords == true`
- sync execute returns result metadata correctly
- async execute returns `202` and persists a queued job
- async worker transitions `queued -> running -> succeeded`
- async worker persists failure details
- startup recovery marks stale running jobs as failed
- JS hook action registration works for both sync and async action definitions

### 10.2 Frontend manual QA

Verify:

- records page shows actions bar below the search bar
- records page no longer shows the bottom bulkbar
- logs page still keeps its current bulkbar behavior
- selection count updates correctly
- `Go` disables correctly
- `Reset` clears selection
- delete built-in still deletes correctly
- export JSON built-in still downloads correctly
- sync custom action appears only on matching collections
- async custom action queues, shows progress, and finishes cleanly
- selection clears on collection/filter/sort/reset changes

### 10.3 Regression focus areas

Pay extra attention to:

- record save/delete document events
- list refresh interactions
- responsive layout on smaller screens
- view collections hiding delete
- action execution while the list is reloading

## 11. File-Level Implementation Map

### 11.1 Backend

New or changed files:

- `core/app.go`
- `core/base.go`
- `core/events.go`
- `core/collection_action.go`
- `core/collection_action_registry.go`
- `core/collection_action_job.go`
- `core/collection_action_job_query.go`
- `apis/base.go`
- `apis/record_actions.go`
- `apis/action_jobs.go`
- `plugins/jsvm/binds.go`
- `plugins/jsvm/internal/types/generated/types.d.ts`
- `migrations/<timestamp>_aux_action_jobs.go`

Recommended tests:

- `core/collection_action_registry_test.go`
- `apis/record_actions_test.go`
- `apis/action_jobs_test.go`
- `plugins/jsvm/binds_test.go`

### 11.2 Frontend

New or changed files:

- `ui/src/collections/pageCollections.js`
- `ui/src/records/recordsList.js`
- `ui/src/records/recordsActionsBar.js`
- `ui/src/css/recordsActions.css`
- `ui/src/main.js`
- `ui/src/css/_main.css`

Optional helper files:

- `ui/src/records/recordsActionsApi.js`
- `ui/src/records/recordsActionJobs.js`
- `ui/src/css/recordsActionJobs.css`

Important caution:

- do not delete `ui/src/css/bulkbar.css` unless every other consumer has been audited

## 12. Recommended Micro-Commit Rollout

The feature is large enough that it should be implemented in small committable slices. Each step below should leave the app in a working state.

### Step 1. Move records bulk actions into a top actions bar

Scope:

- lift `bulkSelected` into `pageCollections`
- refactor `recordsList` into a controlled selection component
- add `recordsActionsBar`
- move built-in delete/export UI into dropdown + `Go` + `Reset`
- remove the records-page bottom bulkbar

Manual verification:

1. Open a normal collection with records.
2. Select multiple rows and confirm the new actions bar appears below the search controls.
3. Choose `Delete selected records`, click `Go`, and confirm the existing confirm modal still appears.
4. Cancel once, then run it again and confirm records are deleted.
5. Choose `Export selected records as JSON` and confirm download still works.
6. Open logs and confirm the logs bulkbar still behaves as before.

Suggested commit message:

`Move record bulk actions into a top collection actions bar`

### Step 2. Add the server-side collection action registry and list API

Scope:

- add core action types and registry
- add `GET /api/collections/{collection}/records/actions`
- wire frontend loading of custom actions into the new bar
- keep built-ins frontend-only

Manual verification:

1. Start the app without any registered custom actions and confirm the records actions bar still shows the built-ins.
2. Hit the list endpoint as a superuser and confirm it returns an empty array when no custom actions are registered.
3. Register one simple Go action and confirm it appears in the dropdown for the intended collection only.

Suggested commit message:

`Add collection action registry and listing endpoint`

### Step 3. Add synchronous custom action execution

Scope:

- add `POST /api/collections/{collection}/records/actions/{action}`
- validate selection rules
- execute sync handlers
- return result metadata to the frontend

Manual verification:

1. Register a simple sync Go action such as “mark reviewed”.
2. Select records, run the action, and confirm the records change as expected.
3. Confirm success toast, selection clearing, and list refresh follow the response metadata.
4. Confirm invalid payloads and unknown action names return clean API errors.

Suggested commit message:

`Execute custom collection actions synchronously`

### Step 4. Expose collection action registration to JS hooks

Scope:

- add `collectionActionAdd(...)` and `collectionActionRemove(...)`
- expose action event fields and progress helpers to JS
- regenerate JS types

Manual verification:

1. Add a custom action in `pb_hooks`.
2. Restart the app and confirm the action appears in the dropdown.
3. Run the action and confirm it updates records correctly.
4. Confirm a JS action marked `executionMode: "sync"` executes inline and returns the expected result.

Suggested commit message:

`Expose collection action registration to JS hooks`

### Step 5. Add durable async collection action jobs

Scope:

- add `_action_jobs` auxiliary persistence
- add enqueue and worker execution
- add `GET /api/action-jobs` and `GET /api/action-jobs/{id}`
- add startup recovery for stale running jobs

Manual verification:

1. Register a custom async action that takes a noticeable amount of time.
2. Run it and confirm the execute endpoint returns `202 Accepted` with a job id.
3. Poll the job detail endpoint and confirm `queued -> running -> succeeded`.
4. Run another async action, restart the app while it is running, and confirm the stale running job is marked failed with an interruption message.

Suggested commit message:

`Add durable async collection action jobs`

### Step 6. Integrate async job polling into the collections UI

Scope:

- show recent jobs under the actions bar
- poll active jobs
- show terminal success/failure feedback
- refresh records on successful async completion when required

Manual verification:

1. Run an async action and confirm the job appears under the actions bar.
2. Confirm status updates from `queued` to `running` to a terminal state.
3. Confirm progress text updates when the handler reports progress.
4. Confirm a successful async action refreshes the list when configured.
5. Reload the page and confirm recent jobs for the active collection can still be loaded from the API.

Suggested commit message:

`Integrate async collection action jobs into the collections UI`

### Step 7. Final docs, examples, and cleanup

Scope:

- add implementation docs and usage examples
- document Go and JS registration patterns
- clean up naming, CSS, and edge-case handling

Manual verification:

1. Read the docs with a fresh local checkout mindset and confirm setup and examples are sufficient.
2. Confirm example Go and JS actions still work after cleanup changes.
3. Re-run the manual smoke test for built-in delete, built-in export, one sync action, and one async action.

Suggested commit message:

`Document collection admin actions and example integrations`

## 13. Recommendation Summary

The recommended implementation is:

- move records selection ownership to `pageCollections`
- replace the records-page bottom bulkbar with a top `recordsActionsBar`
- keep built-in delete/export as frontend actions with their current behavior unchanged
- add a typed server-side collection action registry for Go and JS hook registration
- support sync custom actions directly through the execute endpoint
- support async custom actions through a durable `_action_jobs` subsystem in `auxiliary.db`
- add recent async job visibility directly under the actions bar

This design fits PocketBase well because it:

- preserves current built-in behavior
- keeps the UI refactor separate from action behavior changes
- follows existing app, route, and event patterns
- treats async as a real durable subsystem instead of a best-effort goroutine
- leaves room for future intermediate pages and parameterized actions without destabilizing the initial feature
