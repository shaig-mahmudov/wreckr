package store

import (
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type Store interface {
	CreateTarget(target TargetRecord) TargetRecord
	UpdateTarget(id string, target TargetRecord) (TargetRecord, bool)
	DeleteTarget(id string) bool
	GetTarget(id string) (TargetRecord, bool)
	ListTargets() []TargetRecord

	CreateScenario(sc scenario.Scenario) ScenarioRecord
	UpdateScenario(id string, sc scenario.Scenario) (ScenarioRecord, bool)
	GetScenario(id string) (ScenarioRecord, bool)
	ListScenarios() []ScenarioRecord
	ListScenarioVersions(id string) []ScenarioVersionRecord
	GetScenarioVersion(id string, versionNumber int) (ScenarioVersionRecord, bool)

	CreateRun(
		scenarioID string,
		targetID string,
		sc scenario.Scenario,
		versionRef ...ScenarioVersionRecord,
	) RunRecord
  
	MarkRunStarted(id string)
	CompleteRun(id string, rep report.Report)
	CancelRun(id string, rep report.Report)
	ErrorRun(id string, err error)
	GetRun(id string) (RunRecord, bool)
	ListRuns() []RunRecord
	AppendRunEvent(runID string, event runevent.Event) runevent.Event
	ListRunEvents(runID string) []runevent.Event
}
