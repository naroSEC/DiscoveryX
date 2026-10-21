package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/tools"
	"github.com/naroSEC/DiscoveryX/internal/updater"
)

type githubSnapshot struct {
	Domain   string                `json:"domain"`
	Stage    model.StageState      `json:"stage"`
	Findings []model.GitHubFinding `json:"findings"`
	Commands []model.Command       `json:"commands"`
}

// The GitHub worker owns its engine and files. Only immutable serialized
// snapshots cross to the primary engine, keeping web checkpoints independent.
type githubBranch struct {
	mu      sync.Mutex
	payload []byte
	done    chan struct{}
}

func (e *Engine) collectGitHub() {
	if e.githubBranch == nil {
		return
	}
	e.githubBranch.mu.Lock()
	b := append([]byte{}, e.githubBranch.payload...)
	e.githubBranch.mu.Unlock()
	if len(b) == 0 {
		return
	}
	var snapshot githubSnapshot
	if json.Unmarshal(b, &snapshot) != nil {
		return
	}
	for i := range e.State.Stages {
		if e.State.Stages[i].Name == "ghrecon" {
			e.State.Stages[i] = snapshot.Stage
		}
	}
	e.State.Data.GitHub = snapshot.Findings
	for _, command := range snapshot.Commands {
		found := false
		for i := range e.State.Commands {
			if e.State.Commands[i].ID == command.ID {
				e.State.Commands[i] = command
				found = true
				break
			}
		}
		if !found {
			e.State.Commands = append(e.State.Commands, command)
		}
	}
}

func (e *Engine) startGitHub(ctx context.Context, index int, resume bool) error {
	domain, domainErr := tools.RootDomain(e.Config.Target)
	branch := &githubBranch{done: make(chan struct{})}
	e.githubBranch = branch
	sidecar := filepath.Join(e.Dir, "logs", "ghrecon-state.json")
	var previous githubSnapshot
	if resume && storage.ReadJSON(sidecar, &previous) == nil && previous.Domain == domain && previous.Stage.Status == "Completed" && e.verify(previous.Stage.Files) {
		if storage.ReadJSON(filepath.Join(e.Dir, "normalized", "checkpoint_ghrecon.json"), &previous.Findings) == nil {
			branch.payload, _ = json.Marshal(previous)
			close(branch.done)
			e.collectGitHub()
			fmt.Fprintln(e.log, "[ghrecon] Completed (verified independent checkpoint)")
			return e.save()
		}
	}
	started := time.Now().UTC()
	stage := model.StageState{Name: "ghrecon", Status: "Running", Started: started}
	var history []model.Command
	for _, command := range e.State.Commands {
		if command.Tool == "ghrecon" {
			history = append(history, command)
		}
	}
	if len(previous.Commands) > len(history) {
		history = previous.Commands
	}
	for i := range history {
		if history[i].Status == "Running" {
			history[i].Status = "Interrupted"
			history[i].Error = "Previous process ended before completion was recorded"
		}
	}
	worker := &Engine{Config: e.Config, Dir: e.Dir, GitHubToken: e.GitHubToken, Executor: e.Executor, Registry: e.Registry, Out: e.Out, log: e.log, errors: e.errors,
		Meta: Metadata{Tools: map[string]updater.Installation{}}, State: State{Stages: []model.StageState{stage}, Commands: history}}
	worker.saveHook = func() error {
		snapshot := githubSnapshot{Domain: domain, Stage: worker.State.Stages[0], Findings: worker.State.Data.GitHub, Commands: worker.State.Commands}
		b, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		branch.mu.Lock()
		branch.payload = append([]byte{}, b...)
		branch.mu.Unlock()
		return storage.Atomic(sidecar, b)
	}
	fail := func(status, reason string) error {
		worker.State.Stages[0].Status = status
		worker.State.Stages[0].Reason = reason
		worker.State.Stages[0].Duration = time.Since(started)
		fmt.Fprintf(e.log, "[ghrecon] %s: %s\n", status, reason)
		err := worker.save()
		close(branch.done)
		e.collectGitHub()
		if err != nil {
			return err
		}
		return e.save()
	}
	if domainErr != nil {
		return fail("Skipped", domainErr.Error())
	}
	if e.GitHubToken == "" {
		return fail("Skipped", "GitHub token unavailable; use --github-token, --github-token-file, or GITHUB_TOKEN")
	}
	// Installation remains serialized with the main flow to protect the shared
	// manifest. The scanner process itself runs concurrently with web stages.
	installCtx, cancel := context.WithTimeout(ctx, e.Config.StageTimeout)
	installation, err := e.Manager.Ensure(installCtx, "ghrecon", false)
	if err != nil {
		cancel()
		return fail("Failed", err.Error())
	}
	e.Meta.Tools["ghrecon"] = installation
	worker.Meta.Tools["ghrecon"] = installation
	if err = worker.save(); err != nil {
		cancel()
		close(branch.done)
		return err
	}
	e.State.Stages[index] = stage
	if err = e.save(); err != nil {
		cancel()
		close(branch.done)
		return err
	}
	fmt.Fprintf(e.log, "[ghrecon] Running (parallel GitHub discovery: %s)\n", domain)
	go func() {
		defer close(branch.done)
		defer cancel()
		worker.runGitHub(installCtx, installation.Path)
		_ = worker.save()
		st := worker.State.Stages[0]
		fmt.Fprintf(e.log, "[ghrecon] %s: %d records, %s %s\n", st.Status, st.Count, st.Duration.Round(time.Millisecond), st.Reason)
	}()
	return nil
}

func (e *Engine) runGitHub(parent context.Context, executable string) {
	ctx, cancel := context.WithTimeout(parent, e.Config.StageTimeout)
	defer cancel()
	st := &e.State.Stages[0]
	defer func() { st.Duration = time.Since(st.Started) }()
	finish := func(status string, err error) {
		st.Status = status
		if err != nil {
			st.Reason = err.Error()
			if e.GitHubToken != "" {
				st.Reason = strings.ReplaceAll(st.Reason, e.GitHubToken, "[REDACTED]")
			}
		}
	}
	scanner := e.Registry["ghrecon"]
	if scanner == nil {
		finish("Failed", fmt.Errorf("GHRecon adapter not registered"))
		return
	}
	recorder := runner.Recorder{Executor: e.Executor, Stage: "ghrecon", Tool: "ghrecon", Save: e.recordCommand}
	if err := scanner.Validate(ctx, recorder, executable); err != nil {
		finish("Failed", err)
		return
	}
	in := tools.Input{Config: e.Config, RunDir: e.Dir, GitHubToken: e.GitHubToken}
	calls, err := scanner.BuildArgs(in)
	if err != nil {
		finish("Failed", err)
		return
	}
	call := calls[0]
	if err = e.archivePrevious(call.Output); err != nil {
		finish("Failed", err)
		return
	}
	runErr := scanner.Run(ctx, recorder, executable, call, in)
	var warnings []string
	if f, openErr := os.Open(call.Output); openErr == nil {
		warnings = scanner.Parse(f, &e.State.Data, in)
		f.Close()
	} else if runErr == nil {
		runErr = openErr
	}
	st.Count = len(e.State.Data.GitHub)
	if ctx.Err() != nil {
		finish("Interrupted", ctx.Err())
		return
	}
	var exit *exec.ExitError
	if runErr != nil {
		if errors.As(runErr, &exit) && exit.ExitCode() == 2 {
			finish("Partial", fmt.Errorf("GHRecon returned partial completion (exit 2); findings retained; inspect stderr log"))
		} else {
			finish("Failed", fmt.Errorf("GHRecon failed: %w (see raw/ghrecon.jsonl.stderr.log)", runErr))
		}
		return
	}
	if len(warnings) > 0 {
		finish("Partial", fmt.Errorf("GHRecon parse warnings: %s", strings.Join(warnings, "; ")))
		return
	}
	checkpoint := filepath.Join(e.Dir, "normalized", "checkpoint_ghrecon.json")
	if err = storage.JSON(checkpoint, e.State.Data.GitHub); err != nil {
		finish("Failed", err)
		return
	}
	st.Files = map[string]string{}
	for _, p := range []string{call.Output, call.Output + ".stdout.log", call.Output + ".stderr.log", checkpoint} {
		hash, hashErr := storage.Hash(p)
		if hashErr != nil {
			finish("Failed", hashErr)
			return
		}
		rel, _ := filepath.Rel(e.Dir, p)
		st.Files[filepath.ToSlash(rel)] = hash
	}
	finish("Completed", nil)
}
