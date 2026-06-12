package runevent

import "time"

type Type string

const (
	TypeRunQueued         Type = "run_queued"
	TypeRunStarted        Type = "run_started"
	TypeSetupStarted      Type = "setup_started"
	TypeSetupCompleted    Type = "setup_completed"
	TypeTeardownStarted   Type = "teardown_started"
	TypeTeardownCompleted Type = "teardown_completed"
	TypeRequestStarted    Type = "request_started"
	TypeRequestCompleted  Type = "request_completed"
	TypeAssertionFailed   Type = "assertion_failed"
	TypeInvariantFailed   Type = "invariant_failed"
	TypeThresholdFailed   Type = "threshold_failed"
	TypeCancelRequested   Type = "cancel_requested"
	TypeRunCompleted      Type = "run_completed"
	TypeRunFailed         Type = "run_failed"
	TypeRunCanceled       Type = "run_canceled"
)

const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

type Event struct {
	ID        string         `json:"id,omitempty"`
	RunID     string         `json:"run_id"`
	Sequence  int64          `json:"sequence"`
	Level     string         `json:"level"`
	Type      Type           `json:"type"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Recorder interface {
	Record(Event)
}

type RecorderFunc func(Event)

func (f RecorderFunc) Record(event Event) {
	if f != nil {
		f(event)
	}
}
