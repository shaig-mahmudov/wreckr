package store

import (
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type Store interface {
	CreateScenario(sc scenario.Scenario) ScenarioRecord
	UpdateScenario(id string, sc scenario.Scenario) (ScenarioRecord, bool)
	GetScenario(id string) (ScenarioRecord, bool)
	ListScenarios() []ScenarioRecord
	ListScenarioVersions(id string) []ScenarioVersionRecord

	CreateRun(scenarioID string, sc scenario.Scenario) RunRecord
	MarkRunStarted(id string)
	CompleteRun(id string, rep report.Report)
	ErrorRun(id string, err error)
	GetRun(id string) (RunRecord, bool)
	ListRuns() []RunRecord
}
