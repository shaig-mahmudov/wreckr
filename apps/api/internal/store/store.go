package store

import (
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type Store interface {
	CreateScenario(sc scenario.Scenario) ScenarioRecord
	GetScenario(id string) (ScenarioRecord, bool)
	ListScenarios() []ScenarioRecord

	CreateRun(scenarioID string, sc scenario.Scenario) RunRecord
	MarkRunStarted(id string)
	CompleteRun(id string, rep report.Report)
	ErrorRun(id string, err error)
	GetRun(id string) (RunRecord, bool)
	ListRuns() []RunRecord
}
