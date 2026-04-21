# Collection Admin Actions Reference

This document is the API and behavior reference for the collection admin actions feature.

It describes what changed from the old bulk-action popover, how the new UI works, what backend endpoints were added, and how Go/JS hooks register custom collection actions.

## 1. What Changed

Before this feature, the collection list supported only the built-in bulk popover actions for selected records:

- `Delete selected`
- `Export JSON`

The new feature replaces that popover-style bulk UI with a Django-admin-like action bar:

- an actions dropdown
- a `Go` button
- a `Reset` button
- a selected-record counter
- a modal for async job progress and final status when background execution is used

The built-in delete/export behavior is preserved, but the UI entry point moved from the popup into the new action bar.

## 2. UI Behavior

### Placement

The action bar is rendered on the collections page between the search bar and the records list.

### Selection Model

- Actions apply only to explicitly selected records.
- Selection is page-scoped and currently follows the records shown in the table.
- The action bar is disabled until at least one record is selected.

### Built-in Actions

- `Delete selected records`
- `Export selected records as JSON`

These remain client-side actions.

### Custom Actions

Custom actions are loaded from the backend and rendered in the same dropdown.

The UI only passes selected record ids to the backend. There is no extra input form in v1.

### Async Job UI

Async actions create durable job records in the auxiliary database.

The UI opens a dedicated modal for the queued job so the operator can see:

- the action label
- the current job status
- processed and total counts
- the latest status message or error

This keeps the collection toolbar compact while still making async execution visible and easy to follow.

When a background job is still running after the modal is closed, the action bar shows a compact info indicator for that collection scope so the operator can reopen the status modal later.

## 3. Backend API

## 3.1 List Available Actions

`GET /api/collections/{collection}/records/actions`

Returns the action definitions visible for the requested collection.

Example response:

```json
[
  {
    "name": "publish",
    "label": "Publish selected records",
    "description": "",
    "icon": "",
    "order": 0,
    "collections": ["posts"],
    "excludeCollections": [],
    "selectionRequired": true,
    "minSelection": 1,
    "maxSelection": 0,
    "confirmText": "",
    "variant": "",
    "executionMode": "sync",
    "loadRecords": true,
    "clearSelection": true,
    "reloadRecords": true
  }
]
```

## 3.2 Execute an Action

`POST /api/collections/{collection}/records/actions/{action}`

Request body:

```json
{
  "ids": ["record_id_1", "record_id_2"],
  "payload": {}
}
```

Response behavior:

- sync actions return `200 OK`
- async actions return `202 Accepted`

The response may include:

- `message`
- `clearSelection`
- `reloadRecords`
- `job` for async actions

## 3.3 List Jobs

`GET /api/action-jobs?collection={collection}&status={status}&limit={limit}`

Returns recent action jobs, typically for the current collection.

## 3.4 View a Job

`GET /api/action-jobs/{id}`

Returns a single job record.

## 4. Hook API

Custom actions are registered from `pb_hooks` using:

```js
collectionActionAdd(definition, handler)
collectionActionRemove(name)
```

The canonical TypeScript declarations live in
[plugins/jsvm/internal/types/generated/types.d.ts](./plugins/jsvm/internal/types/generated/types.d.ts).
The main types are:

- `core.CollectionActionDefinition`
- `core.CollectionActionRequestEvent`
- `core.CollectionActionResult`
- `core.CollectionActionJob`

### 4.1 Typed Definition

`collectionActionAdd(definition, handler)` accepts `core.CollectionActionDefinition`.

Field | Type | Required | Notes
---|---|---|---
`name` | `string` | yes | Unique action name.
`label` | `string` | no | Defaults to `name` if omitted.
`description` | `string` | no | Optional helper text.
`icon` | `string` | no | Optional icon identifier used by the UI.
`order` | `number` | no | Sort order in the action dropdown.
`collections` | `string[]` | no | Limit the action to specific collections. Empty means all collections unless excluded.
`excludeCollections` | `string[]` | no | Exclude specific collection ids or names.
`selectionRequired` | `boolean` | no | Enforces at least one selected record.
`minSelection` | `number` | no | Enforces a minimum selected count.
`maxSelection` | `number` | no | Enforces a maximum selected count.
`confirmText` | `string` | no | Optional confirmation message shown before execution.
`variant` | `string` | no | UI styling hint, like `success` or `warning`.
`executionMode` | `"sync" \| "async"` | no | Defaults to `sync`.
`loadRecords` | `boolean` | no | Set this to `true` when the handler needs `e.records`.
`clearSelection` | `boolean` | no | Requests the UI to clear the selection after success.
`reloadRecords` | `boolean` | no | Requests the UI to refresh the table after success.

In a `.pb.ts` hook you can annotate the object directly:

```ts
const publishAction: core.CollectionActionDefinition = {
  name: "publish_posts",
  label: "Publish selected posts",
  collections: ["posts"],
  executionMode: "sync",
  loadRecords: true,
}
```

In a `.pb.js` hook you can get the same IntelliSense with JSDoc:

```js
/** @type {core.CollectionActionDefinition} */
const publishAction = {
  name: "publish_posts",
  label: "Publish selected posts",
  collections: ["posts"],
  executionMode: "sync",
  loadRecords: true,
}
```

### 4.2 Event Fields

The handler receives a `core.CollectionActionRequestEvent` object with:

Field | Type | Notes
---|---|---
`app` | `core.App` | Current PocketBase app instance.
`collection` | `core.Collection \| undefined` | The resolved collection context.
`action` | `core.CollectionActionDefinition` | Normalized action definition.
`recordIds` | `string[]` | The selected record ids.
`records` | `core.Record[] \| undefined` | Loaded only when `loadRecords` is enabled.
`payload` | `Record<string, any>` | Submitted request payload.
`job` | `core.CollectionActionJob \| undefined` | Present for async execution.
`result` | `core.CollectionActionResult \| undefined` | Mutable response data for sync handlers.

The event also exposes helper methods:

- `setProgress(processedItems, totalItems, message)` for async jobs
- `setResultMessage(message)`

### 4.3 Result and Job Shapes

`core.CollectionActionResult` may include:

- `message`
- `data`
- `clearSelection`
- `reloadRecords`
- `job`

`core.CollectionActionJob` is the durable async job record returned by the API and shown in the modal. It includes:

- `id`
- `actionName`
- `actionLabel`
- `collectionId`
- `collectionName`
- `status`
- `recordIds`
- `payload`
- `result`
- `error`
- `processedItems`
- `totalItems`
- `statusMessage`
- `started`
- `finished`
- `created`
- `updated`

## 5. Execution Modes

### `sync`

- runs immediately during the API request
- best for quick, deterministic updates
- should be used when the action completes in a short time

### `async`

- enqueues a durable job in `_action_jobs`
- runs in the background
- suitable for longer operations or actions that should survive restarts

## 6. Example Collection: `posts`

For the following collection schema:

- collection name: `posts`
- status field: select with values `pending` and `approved`

the recommended example hooks are:

- a sync action that sets selected posts to `pending`
- an async action that sets selected posts to `approved`
- a sync action that sets selected posts as published
- a sync action that sets selected posts as unpublished

## 7. Behavioral Differences From The Old Bulk Popup

- The old popup was action-specific and modal-like.
- The new UI is always visible as part of the collection toolbar area.
- The old UI had no hook-driven custom action registry.
- The new feature supports Go and JS hook registration.
- The old UI had no durable async job model.
- The new feature can show action jobs and recover interrupted background work.

## 8. Implementation Notes

- Built-in delete/export remain client-side to preserve their current behavior.
- Custom actions are backend-driven.
- The UI currently passes selected ids only.
- There is no extra parameter form yet.
- Async actions should be used when the work may take long enough to degrade the request/response flow.
- Async job status is shown in a modal instead of an inline “recent jobs” strip to keep the page layout clean.

## 9. File References

- UI action bar: [ui/src/records/recordsActionsBar.js](./ui/src/records/recordsActionsBar.js)
- Collection page integration: [ui/src/collections/pageCollections.js](./ui/src/collections/pageCollections.js)
- Backend action API: [apis/record_actions.go](./apis/record_actions.go)
- Job API: [apis/action_jobs.go](./apis/action_jobs.go)
- Collection action core types: [core/collection_action.go](./core/collection_action.go)
- Async job model: [core/collection_action_job.go](./core/collection_action_job.go)
- JS hook binding: [plugins/jsvm/binds.go](./plugins/jsvm/binds.go)
- Example hooks: [examples/base/pb_hooks/collection_admin_actions.pb.js](./examples/base/pb_hooks/collection_admin_actions.pb.js)
