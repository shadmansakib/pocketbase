package core

import (
	"log/slog"

	"github.com/pocketbase/pocketbase/tools/hook"
)

func (app *BaseApp) registerCollectionActionHooks() {
	app.OnServe().Bind(&hook.Handler[*ServeEvent]{
		Id: "__pbCollectionActionJobsRecover__",
		Func: func(e *ServeEvent) error {
			if err := RecoverCollectionActionJobs(e.App); err != nil {
				e.App.Logger().Warn(
					"Failed to recover queued collection action jobs",
					slog.String("error", err.Error()),
				)
			}

			return e.Next()
		},
		Priority: 998,
	})
}
