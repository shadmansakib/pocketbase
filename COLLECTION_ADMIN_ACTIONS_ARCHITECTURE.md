# Collection Admin Actions Architecture

This document is the implementation source of truth for the collection admin actions feature in this PocketBase fork.

Collection admin actions are intentionally synchronous. PocketBase's JavaScript hook runtime does not support Promise-based async handlers, and this feature should not pretend otherwise with an action-specific background job layer.

## 1. Goals

- Replace the selected-record bulk popover with a Django-admin-style action bar.
- Keep the existing built-in delete/export behavior.
- Allow Go and JS hook code to register custom selected-record actions.
- Keep the action contract small, predictable, and aligned with PocketBase hooks.
- Avoid durable action-job persistence, polling, and modal status UI in this feature.

## 2. Non-goals

- No "select all matching filter across pages" behavior.
- No intermediate pages or extra action input in v1.
- No Promise/async JavaScript handler support.
- No `_action_jobs` table, job API, job recovery, polling, or progress modal.

Long-running work should be modeled as a separate app-specific job/cron/queue feature. A collection action may trigger or mark that work, but the action itself completes synchronously.

## 3. UI Architecture

The action bar lives between the collection search bar and the records list.

```text
collection page
  -> search bar
  -> records actions bar
       -> selected count
       -> action dropdown
       -> Go button
       -> Reset button
  -> records list
```

Selection state is owned by the collection page and passed to the records list and actions bar. This replaces the old list-local bulk popover state and lets the action bar remain visible in a predictable place.

Built-in actions stay client-side:

- `Delete selected records`
- `Export selected records as JSON`

Custom actions are fetched from:

```text
GET /api/collections/{collection}/records/actions
```

Custom actions are executed through:

```text
POST /api/collections/{collection}/records/actions/{action}
```

The request contains selected record ids only:

```json
{
  "ids": ["record_id_1", "record_id_2"],
  "payload": {}
}
```

## 4. Backend Architecture

The backend has three parts:

- `core.CollectionActionRegistry`
- record action API routes in `apis/record_actions.go`
- JSVM registration helpers in `plugins/jsvm/binds.go`

```text
JS hook / Go code
  -> app.CollectionActions().Add(...)
  -> registry stores normalized action
  -> UI lists actions for current collection
  -> operator chooses action and clicks Go
  -> API validates selected ids
  -> optional records are loaded
  -> handler runs synchronously
  -> JSON response drives UI clear/reload/toast behavior
```

## 5. Core Types

`core.CollectionActionDefinition` aliases `core.CollectionAction` so Go and JSVM docs can use one public name.

Important fields:

- `Name`
- `Label`
- `Description`
- `Icon`
- `Order`
- `Collections`
- `ExcludeCollections`
- `SelectionRequired`
- `MinSelection`
- `MaxSelection`
- `ConfirmText`
- `Variant`
- `LoadRecords`
- `ClearSelection`
- `ReloadRecords`
- `Handler`

There is no `ExecutionMode`.

`core.CollectionActionRequestEvent` exposes:

- `App`
- `RequestEvent`
- `Collection`
- `Action`
- `RecordIds`
- `Records`
- `Payload`
- `Result`

There is no `Job` field and no progress API.

## 6. JS Hook API

Hooks register actions with:

```js
collectionActionAdd(definition, handler)
collectionActionRemove(name)
```

Example:

```js
collectionActionAdd({
    name: "publish_selected_records",
    label: "Publish selected records",
    collections: ["shopping_items"],
    minSelection: 1,
    loadRecords: true,
    clearSelection: true,
    reloadRecords: true,
}, function(e) {
    e.app.runInTransaction(function(txApp) {
        for (const record of e.records || []) {
            const txRecord = txApp.findRecordById(e.collection.id, record.id);
            txRecord.set("published", true);
            txApp.save(txRecord);
        }
    });

    e.setResultMessage("Selected records were published.");
});
```

`executionMode` must not be used in hook definitions.

## 7. Response Contract

Successful custom actions return `200 OK`.

The response may include:

- `message`
- `data`
- `clearSelection`
- `reloadRecords`

Errors are returned as normal PocketBase API errors. The UI uses the existing toast/error handling.

## 8. Files

- `core/collection_action.go`
- `core/collection_action_registry.go`
- `apis/record_actions.go`
- `plugins/jsvm/binds.go`
- `plugins/jsvm/internal/types/types.go`
- `plugins/jsvm/internal/types/generated/types.d.ts`
- `ui/src/collections/pageCollections.js`
- `ui/src/records/recordsActionsBar.js`
- `ui/src/records/recordsList.js`
- `ui/src/css/recordsActions.css`

Removed async/job files must stay removed:

- `apis/action_jobs.go`
- `core/collection_action_job.go`
- `core/collection_action_job_query.go`
- `core/collection_action_hooks.go`
- `migrations/*_aux_collection_action_jobs.go`
- `ui/src/records/collectionActionJobModal.js`

## 9. Verification

Manual smoke tests:

- Select records and export JSON.
- Select records and delete them.
- Register a JS hook action without `executionMode`.
- Confirm the action appears in the dropdown after restart.
- Run the action and confirm it returns immediately with a success toast.
- Confirm `GET /api/action-jobs` is not registered.

Automated checks:

- `go test ./core ./apis ./plugins/jsvm`
- `go run ./plugins/jsvm/internal/types`
- `cd ui && npm run build`

