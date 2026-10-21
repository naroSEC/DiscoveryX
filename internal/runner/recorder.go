package runner

import (
	"context"
	"errors"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"runtime"
	"sync/atomic"
	"time"
)

var commandSequence atomic.Uint64

type Recorder struct {
	Executor    Executor
	Stage, Tool string
	Save        func(model.Command) error
}

func (r Recorder) Run(ctx context.Context, req Request) error {
	purpose := "inspect"
	for _, arg := range req.Args {
		if arg == "-o" || arg == "--jsonl" {
			purpose = "scan"
			break
		}
		if arg == "-tl" || arg == "-ut" {
			purpose = "templates"
		}
	}
	started := time.Now().UTC()
	entry := model.Command{ID: fmt.Sprintf("%d-%d", started.UnixNano(), commandSequence.Add(1)), Stage: r.Stage, Tool: r.Tool, Purpose: purpose, Platform: runtime.GOOS, Executable: req.Executable, Args: append([]string{}, req.Args...), Directory: req.Dir, Started: started, Status: "Running"}
	if f, ok := req.Stdout.(interface{ Name() string }); ok {
		entry.Stdout = f.Name()
	}
	if f, ok := req.Stderr.(interface{ Name() string }); ok {
		entry.Stderr = f.Name()
	}
	if r.Save != nil {
		if err := r.Save(entry); err != nil {
			return fmt.Errorf("save command start: %w", err)
		}
	}
	err := r.Executor.Run(ctx, req)
	entry.Duration = time.Since(started)
	entry.Status = "Completed"
	if err != nil {
		entry.Status = "Failed"
		entry.Error = err.Error()
		if ctx.Err() != nil {
			entry.Status = "Canceled"
		}
	}
	if r.Save != nil {
		if saveErr := r.Save(entry); saveErr != nil {
			return errors.Join(err, fmt.Errorf("save command result: %w", saveErr))
		}
	}
	return err
}
