window.app = window.app || {};
window.app.components = window.app.components || {};

const builtinRecordActions = [
    {
        name: "export_json",
        label: "Export selected records as JSON",
        icon: "ri-download-line",
        type: "export",
        confirmText: "",
    },
    {
        name: "delete_selected",
        label: "Delete selected records",
        icon: "ri-delete-bin-7-line",
        type: "delete",
        confirmText: "Do you really want to delete the selected records?",
        variant: "danger",
    },
];

window.app.components.recordsActionsBar = function(propsArg = {}) {
    const props = store({
        collection: {},
        bulkSelected: {},
        hidden: undefined,
        onbulkselectchange: (selected) => {},
        onrefresh: () => {},
    });

    const propWatchers = app.utils.extendStore(props, propsArg);

    const data = store({
        selectedAction: "",
        remoteActions: [],
        isLoadingActions: false,
        isExecuting: false,
        actionsRequestKey: "",
        get selectedIds() {
            return Object.keys(props.bulkSelected || {});
        },
        get selectedRecords() {
            return Object.values(props.bulkSelected || {});
        },
        get totalSelected() {
            return data.selectedIds.length;
        },
        get actionOptions() {
            const options = [];

            for (const action of builtinRecordActions) {
                if (action.name == "delete_selected" && props.collection?.type == "view") {
                    continue;
                }

                options.push({
                    value: action.name,
                    label: action.label,
                });
            }

            for (const action of data.remoteActions) {
                options.push({
                    value: action.name,
                    label: action.label || action.name,
                });
            }

            return options;
        },
        get selectedActionDef() {
            const builtin = builtinRecordActions.find((a) => a.name == data.selectedAction);
            if (builtin) {
                return builtin;
            }

            return data.remoteActions.find((a) => a.name == data.selectedAction);
        },
        get canRun() {
            return !!data.selectedAction && !!data.totalSelected && !data.isExecuting && !data.isLoadingActions;
        },
    });

    function clearSelection() {
        props.onbulkselectchange?.({});
    }

    function resetSelection() {
        clearSelection();
    }

    function downloadSelected() {
        const selected = JSON.parse(JSON.stringify(data.selectedRecords));
        if (!selected.length) {
            return;
        }

        for (const record of selected) {
            if (record.expand) {
                delete record.expand;
            }
        }

        if (selected.length == 1) {
            return app.utils.downloadJSON(selected[0], props.collection.name + "_" + selected[0].id + ".json");
        }

        return app.utils.downloadJSON(selected, `${selected.length}_${props.collection.name}_records.json`);
    }

    async function deleteSelected() {
        const idsToDelete = data.selectedIds.slice();
        if (!idsToDelete.length) {
            return;
        }

        const remainingIdsToDelete = idsToDelete.slice();

        while (remainingIdsToDelete.length) {
            const ids = remainingIdsToDelete.splice(0, 100);
            const promises = [];
            for (const id of ids) {
                promises.push(app.pb.collection(props.collection.name).delete(id));
            }

            try {
                await Promise.all(promises);
            } catch (err) {
                app.checkApiError(err);
                clearSelection();
                props.onrefresh?.();
                return;
            }
        }

        clearSelection();

        app.toasts.success(
            `Successfully deleted ${idsToDelete.length} ${idsToDelete.length == 1 ? "record" : "records"}.`,
        );
    }

    async function loadRemoteActions() {
        if (!props.collection?.name) {
            data.remoteActions = [];
            return;
        }

        data.isLoadingActions = true;
        data.actionsRequestKey = "records_actions_" + props.collection.name;

        try {
            const result = await app.pb.send(`/api/collections/${props.collection.name}/records/actions`, {
                requestKey: data.actionsRequestKey,
            });

            data.remoteActions = Array.isArray(result) ? result : [];
        } catch (err) {
            if (!err.isAbort) {
                data.remoteActions = [];
                if (err.status && err.status != 404) {
                    console.warn("[recordsActionsBar] failed to load collection actions:", err);
                }
            }
        }

        data.isLoadingActions = false;
    }

    async function executeRemoteAction(action) {
        const response = await app.pb.send(
            `/api/collections/${props.collection.name}/records/actions/${action.name}`,
            {
                method: "POST",
                body: {
                    ids: data.selectedIds,
                },
                requestKey: "records_action_execute_" + props.collection.name + "_" + action.name,
            },
        );

        if (response?.clearSelection) {
            clearSelection();
        }

        if (response?.reloadRecords) {
            props.onrefresh?.();
        }

        if (response?.message) {
            app.toasts.success(response.message);
        } else {
            app.toasts.success("Action completed successfully.");
        }
    }

    async function runSelectedAction() {
        if (!data.canRun || !data.selectedActionDef) {
            return;
        }

        const action = data.selectedActionDef;

        if (action.type == "delete") {
            await app.modals.confirm(action.confirmText, deleteSelected);
            return;
        }

        if (action.type == "export") {
            downloadSelected();
            return;
        }

        data.isExecuting = true;

        try {
            await executeRemoteAction(action);
        } catch (err) {
            app.checkApiError(err);
        }

        data.isExecuting = false;
    }

    const watchers = [
        watch(
            () => props.collection?.id,
            (newVal, oldVal) => {
                data.selectedAction = "";

                if (data.actionsRequestKey) {
                    app.pb.cancelRequest(data.actionsRequestKey);
                }

                if (newVal && newVal != oldVal) {
                    loadRemoteActions();
                } else {
                    data.remoteActions = [];
                }
            },
        ),
        watch(
            () => data.actionOptions.map((a) => a.value).join("|"),
            () => {
                if (data.selectedAction && !data.actionOptions.find((a) => a.value == data.selectedAction)) {
                    data.selectedAction = "";
                }
            },
        ),
    ];

    return t.div(
        {
            pbEvent: "recordsActionsBar",
            hidden: () => props.hidden,
            className: () => `records-actions-wrapper ${data.totalSelected ? "has-selection" : ""}`,
            onmount: () => {
                loadRemoteActions();
            },
            onunmount: () => {
                propWatchers.forEach((w) => w?.unwatch());
                if (data.actionsRequestKey) {
                    app.pb.cancelRequest(data.actionsRequestKey);
                }
            },
        },
        t.div(
            { className: "records-actions-bar" },
            t.div(
                { className: "records-actions-summary" },
                t.span({ className: "txt-hint" }, "Selected "),
                t.strong(null, () => data.totalSelected),
                t.span({ className: "txt-hint" }, () => ` ${data.totalSelected == 1 ? "record" : "records"}`),
            ),
            t.div(
                { className: "records-actions-controls" },
                app.components.select({
                    className: "records-actions-select",
                    placeholder: "Choose an action",
                    options: () => data.actionOptions,
                    value: () => data.selectedAction,
                    onchange: (opts) => {
                        data.selectedAction = opts[0]?.value || "";
                    },
                }),
                t.button(
                    {
                        type: "button",
                        className: () => `btn primary records-actions-go ${data.canRun ? "" : "disabled"}`,
                        disabled: () => !data.canRun,
                        onclick: () => runSelectedAction(),
                    },
                    t.span({ className: "txt" }, "Go"),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn secondary records-actions-reset",
                        disabled: () => !data.totalSelected,
                        onclick: () => resetSelection(),
                    },
                    t.span({ className: "txt" }, "Reset"),
                ),
            ),
        ),
    );
};
