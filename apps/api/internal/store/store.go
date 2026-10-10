package store

import (
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type RunMetrics struct {
	Total    int
	Active   int
	Passed   int
	Failed   int
	Errored  int
	Canceled int
}

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
	GetRunMetrics() RunMetrics
	RequestRunCancel(id string) bool
	IsRunCancelRequested(id string) bool
	AppendRunEvent(runID string, event runevent.Event) runevent.Event
	ListRunEvents(runID string) []runevent.Event
}
