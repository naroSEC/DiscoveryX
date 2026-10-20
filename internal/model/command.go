package model

import "time"

// Command records a process invocation, rather than an inferred command line.
// Arguments stay separate so paths and shell metacharacters remain unambiguous.
type Command struct {
	ID         string        `json:"id"`
	Stage      string        `json:"stage"`
	Tool       string        `json:"tool"`
	Purpose    string        `json:"purpose"`
	Platform   string        `json:"platform"`
	Executable string        `json:"executable"`
	Args       []string      `json:"args"`
	Directory  string        `json:"directory,omitempty"`
	Started    time.Time     `json:"started"`
	Duration   time.Duration `json:"duration_ns"`
	Status     string        `json:"status"`
	Error      string        `json:"error,omitempty"`
	Stdout     string        `json:"stdout,omitempty"`
	Stderr     string        `json:"stderr,omitempty"`
}
