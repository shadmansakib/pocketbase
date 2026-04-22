package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.SystemMigrations.Add(&core.Migration{
		Up: func(txApp core.App) error {
			_, execErr := txApp.AuxDB().NewQuery(`
				CREATE TABLE IF NOT EXISTS {{_action_jobs}} (
					[[id]]             TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
					[[actionName]]     TEXT NOT NULL,
					[[actionLabel]]    TEXT NOT NULL,
					[[collectionId]]   TEXT NOT NULL,
					[[collectionName]] TEXT NOT NULL,
					[[status]]         TEXT NOT NULL,
					[[recordIds]]      JSON DEFAULT "[]" NOT NULL,
					[[payload]]        JSON DEFAULT "{}" NOT NULL,
					[[result]]         JSON DEFAULT "null" NOT NULL,
					[[error]]          TEXT DEFAULT "" NOT NULL,
					[[processedItems]] INTEGER DEFAULT 0 NOT NULL,
					[[totalItems]]     INTEGER DEFAULT 0 NOT NULL,
					[[statusMessage]]  TEXT DEFAULT "" NOT NULL,
					[[started]]        TEXT DEFAULT "" NOT NULL,
					[[finished]]       TEXT DEFAULT "" NOT NULL,
					[[created]]        TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
					[[updated]]        TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
				);

				CREATE INDEX IF NOT EXISTS idx_action_jobs_status on {{_action_jobs}} ([[status]]);
				CREATE INDEX IF NOT EXISTS idx_action_jobs_collection on {{_action_jobs}} ([[collectionId]], [[collectionName]]);
				CREATE INDEX IF NOT EXISTS idx_action_jobs_created on {{_action_jobs}} ([[created]]);
			`).Execute()

			return execErr
		},
		Down: func(txApp core.App) error {
			_, err := txApp.AuxDB().DropTable("_action_jobs").Execute()
			return err
		},
		ReapplyCondition: func(txApp core.App, runner *core.MigrationsRunner, fileName string) (bool, error) {
			return !txApp.AuxHasTable("_action_jobs"), nil
		},
	})
}
