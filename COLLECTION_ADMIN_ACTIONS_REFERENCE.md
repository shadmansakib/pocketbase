# Collection Admin Actions Reference

This document is the API and behavior reference for the collection admin actions feature.

Collection admin actions are synchronous. PocketBase's JSVM hook runtime does not support Promise-based async handlers, so custom actions run during the action API request and return a normal success or error response.

## 1. What Changed

Before this feature, selected records in a collection list opened a small bulk-action popover with:

- `Delete selected`
- `Export JSON`

The feature replaces that popover with a Django-admin-style action bar:

- selected-record counter
- action dropdown
- `Go` button
- `Reset` button

Built-in delete and export still use the existing client-side behavior. Custom actions are registered by Go code or JS hooks and appear in the same dropdown.

## 2. UI Behavior

The action bar is rendered on the collections page between the search bar and the records list.

Actions apply only to explicitly selected records. The UI sends the selected record ids to the backend, and v1 does not support extra action input or intermediate pages.

Built-in actions:

- `Delete selected records`
- `Export selected records as JSON`

Custom actions:

- Loaded from `GET /api/collections/{collection}/records/actions`.
- Executed by `POST /api/collections/{collection}/records/actions/{action}`.
- Run synchronously and return immediately when the handler completes.
- May request that the UI clears selection or reloads the records table.

## 3. Backend API

### 3.1 List Available Actions

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
    "loadRecords": true,
    "clearSelection": true,
    "reloadRecords": true
  }
]
```

### 3.2 Execute an Action

`POST /api/collections/{collection}/records/actions/{action}`

Request body:

```json
{
  "ids": ["record_id_1", "record_id_2"],
  "payload": {}
}
```

Successful actions return `200 OK`.

Example response:

```json
{
  "message": "Publish selected records completed successfully.",
  "clearSelection": true,
  "reloadRecords": true
}
```

There are no action-job endpoints and no `_action_jobs` persistence table.

## 4. Hook API

Custom actions are registered from `pb_hooks` using:

```js
collectionActionAdd(definition, handler)
collectionActionRemove(name)
```

The canonical TypeScript declarations live in
[plugins/jsvm/internal/types/generated/types.d.ts](./plugins/jsvm/internal/types/generated/types.d.ts).

Main types:

- `core.CollectionActionDefinition`
- `core.CollectionActionRequestEvent`
- `core.CollectionActionResult`

### 4.1 Definition Fields

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
`loadRecords` | `boolean` | no | Set this to `true` when the handler needs `e.records`.
`clearSelection` | `boolean` | no | Requests the UI to clear the selection after success.
`reloadRecords` | `boolean` | no | Requests the UI to refresh the table after success.

The JSVM parser defaults to:

- `selectionRequired: true`
- `minSelection: 1`
- `loadRecords: true`
- `clearSelection: true`
- `reloadRecords: true`

### 4.2 TypeScript Example

```ts
const publishAction: core.CollectionActionDefinition = {
  name: "publish_posts",
  label: "Publish selected posts",
  collections: ["posts"],
  loadRecords: true,
}

collectionActionAdd(publishAction, (e) => {
  for (const record of e.records || []) {
    record.set("published", true)
    e.app.save(record)
  }

  e.setResultMessage("Selected posts were published.")
})
```

### 4.3 JavaScript Example

```js
/** @type {core.CollectionActionDefinition} */
const publishAction = {
  name: "publish_posts",
  label: "Publish selected posts",
  collections: ["posts"],
  loadRecords: true,
}

collectionActionAdd(publishAction, function(e) {
  for (const record of e.records || []) {
    record.set("published", true)
    e.app.save(record)
  }

  e.setResultMessage("Selected posts were published.")
})
```

### 4.4 Event Fields

The handler receives a `core.CollectionActionRequestEvent` object with:

Field | Type | Notes
---|---|---
`app` | `core.App` | Current PocketBase app instance.
`collection` | `core.Collection \| undefined` | The resolved collection context.
`action` | `core.CollectionActionDefinition` | Normalized action definition.
`recordIds` | `string[]` | The selected record ids.
`records` | `core.Record[] \| undefined` | Loaded only when `loadRecords` is enabled.
`payload` | `Record<string, any>` | Submitted request payload.
`result` | `core.CollectionActionResult \| undefined` | Mutable response data.

The event also exposes:

- `setResultMessage(message)`

### 4.5 Result Shape

`core.CollectionActionResult` may include:

- `message`
- `data`
- `clearSelection`
- `reloadRecords`

Handlers can either set `e.result` directly or call `e.setResultMessage(...)`.

## 5. Execution Contract

All custom actions run synchronously.

Guidelines:

- Keep handlers quick and deterministic.
- Use `loadRecords: false` if the action only needs selected ids.
- Use `e.app.runInTransaction(...)` for grouped writes.
- Use the transaction callback app, not the outer app, inside transactions.
- For long-running work, register a separate job/cron/queue feature outside collection actions and let the action only trigger or mark that work.

## 6. Built-in Example Hooks

The example app includes selected-record actions in `examples/base/pb_hooks`:

- publish selected shopping items
- unpublish selected shopping items
- mark selected posts pending
- approve selected posts

All examples are synchronous and should omit `executionMode`.

## 7. Files

- Backend action API: [apis/record_actions.go](./apis/record_actions.go)
- Registry and event types: [core/collection_action.go](./core/collection_action.go)
- Registry implementation: [core/collection_action_registry.go](./core/collection_action_registry.go)
- JS hook bindings: [plugins/jsvm/binds.go](./plugins/jsvm/binds.go)
- TypeScript declarations: [plugins/jsvm/internal/types/generated/types.d.ts](./plugins/jsvm/internal/types/generated/types.d.ts)
- UI action bar: [ui/src/records/recordsActionsBar.js](./ui/src/records/recordsActionsBar.js)

