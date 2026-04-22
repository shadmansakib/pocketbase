window.app = window.app || {};
window.app.modals = window.app.modals || {};

window.app.modals.openCollectionActionJob = function(job, settings = {}) {
    const modal = collectionActionJobModal(job, settings);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
};

function collectionActionJobModal(initialJob, settings) {
    if (!initialJob?.id) {
        return null;
    }

    let modal;
    let pollerId;

    const data = store({
        job: initialJob,
        isLoading: false,
    });

    function isTerminal(job) {
        return job?.status == "succeeded" || job?.status == "failed";
    }

    function progressPercent(job) {
        const total = job?.totalItems || 0;
        if (total <= 0) {
            return 0;
        }

        const processed = job?.processedItems || 0;
        return Math.max(0, Math.min(100, Math.round((processed / total) * 100)));
    }

    function stopPolling() {
        clearInterval(pollerId);
        pollerId = undefined;
    }

    async function loadJob() {
        if (!data.job?.id) {
            return;
        }

        try {
            const result = await app.pb.send(`/api/action-jobs/${data.job.id}`, {
                requestKey: "collection_action_job_" + data.job.id,
            });

            if (result?.id) {
                data.job = result;
            }

            if (isTerminal(data.job)) {
                stopPolling();
                settings.oncomplete?.(data.job);
            }
        } catch (err) {
            if (!err.isAbort) {
                console.warn("[collectionActionJobModal] failed to load job:", err);
            }
        }
    }

    function syncPolling() {
        stopPolling();

        if (!isTerminal(data.job)) {
            pollerId = setInterval(() => {
                loadJob();
            }, 2500);
        }
    }

    modal = t.div(
        {
            className: "modal popup sm collection-action-job-modal",
            onmount: () => {
                loadJob();
                syncPolling();
            },
            onunmount: () => {
                stopPolling();
                if (data.job?.id) {
                    app.pb.cancelRequest("collection_action_job_" + data.job.id);
                }
            },
        },
        t.header(
            { className: "modal-header isolated" },
            t.div(
                { className: "collection-action-job-titlewrap" },
                t.h5({ className: "modal-title" }, () => data.job.actionLabel || data.job.actionName || "Action job"),
                t.div({ className: "txt-hint collection-action-job-subtitle" }, () => {
                    const collectionName = data.job.collectionName || "collection";
                    const jobId = data.job.id || "";
                    return `${collectionName} · ${jobId}`;
                }),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn sm transparent circle modal-close-btn",
                    ariaLabel: app.attrs.tooltip("Close modal"),
                    onclick: () => app.modals.close(modal),
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                { className: "collection-action-job-card" },
                t.div(
                    { className: "collection-action-job-statusrow" },
                    t.span({ className: "txt-hint" }, "Status"),
                    t.span(
                        { className: () => `status-badge job-${data.job.status || "unknown"}` },
                        () => data.job.status || "unknown",
                    ),
                ),
                t.div(
                    { className: "collection-action-job-progress" },
                    t.div(
                        { className: "collection-action-job-progress-head" },
                        t.span({ className: "txt-hint" }, "Progress"),
                        t.strong(null, () => {
                            const processed = data.job.processedItems ?? 0;
                            const total = data.job.totalItems ?? 0;
                            return `${processed}/${total}`;
                        }),
                    ),
                    t.div(
                        { className: "collection-action-job-progressbar", "aria-hidden": true },
                        t.div(
                            {
                                className: "collection-action-job-progressbar-fill",
                                style: () => `width: ${progressPercent(data.job)}%;`,
                            },
                        ),
                    ),
                ),
                t.div(
                    { className: "collection-action-job-message txt-hint" },
                    () => data.job.statusMessage || "",
                ),
            ),
            t.div(
                { className: "collection-action-job-outcome", hidden: () => !isTerminal(data.job) },
                t.div(
                    {
                        className: () => `field-help ${data.job.status == "failed" ? "error" : "success"}`,
                    },
                    () =>
                        data.job.status == "failed"
                            ? "The action job failed."
                            : "The action job completed successfully.",
                ),
            ),
            t.div(
                { hidden: () => !data.job.error },
                t.div({ className: "field-help error" }, () => data.job.error || ""),
            ),
            t.div(
                { className: "collection-action-job-footnote", hidden: () => isTerminal(data.job) },
                t.span({ className: "loader sm" }),
                t.span({ className: "txt-hint m-l-sm" }, "The background job is still running."),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn secondary",
                    onclick: () => {
                        stopPolling();
                        app.modals.close(modal);
                    },
                },
                t.span({ className: "txt" }, "Close"),
            ),
        ),
    );

    return modal;
}
