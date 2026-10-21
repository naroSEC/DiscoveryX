package engine

import (
	"context"
	"errors"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/testutil"
)

func githubFixture(t *testing.T) *Engine {
	t.Helper()
	e := webOnlyFixture(t, testutil.Binary(t))
	e.Config.Crawl = "on"
	e.Config.GHRecon = "on"
	e.GitHubToken = "github_pat_OFFLINE_TEST_SECRET_ONLY"
	e.Meta.Config = e.Config
	e.State.Stages = nil
	for _, stage := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, model.StageState{Name: stage.Name, Status: "Pending"})
	}
	return e
}

func TestGitHubParallelAndIndependentResume(t *testing.T) {
	e := githubFixture(t)
	t.Setenv("MOCK_GH_BARRIER", filepath.Join(t.TempDir(), "parallel"))
	t.Setenv("MOCK_GH_SECRET_STDERR", "1")
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.Meta.State != "Completed" || len(e.State.Data.GitHub) != 1 {
		t.Fatalf("%s %+v", e.Meta.State, e.State.Stages)
	}
	// The child processes wait for each other's marker. Sequential execution
	// deadlocks until the stage timeout and cannot pass this assertion.
	if e.State.Stages[1].Name != "ghrecon" || e.State.Stages[2].Name != "katana" {
		t.Fatal(e.State.Stages)
	}
	if e.State.Data.GitHub[0].Value != "github.fixture.test" {
		t.Fatal(e.State.Data.GitHub)
	}
	for _, asset := range e.State.Data.Assets {
		if asset.Hostname == "github.fixture.test" {
			t.Fatal("OSINT result was scanned")
		}
	}
	err := filepath.WalkDir(e.Dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), e.GitHubToken) {
			t.Fatalf("token leaked to %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(e.State.Commands)
	// Simulate a completed worker whose main stage status was not saved before
	// a process interruption. The worker's verified checkpoint must recover it.
	crashState := e.State
	crashState.Stages = append([]model.StageState{}, e.State.Stages...)
	crashState.Stages[1].Status = "Running"
	crashState.Data.GitHub = nil
	if err = storage.JSON(filepath.Join(e.Dir, "status.json"), crashState); err != nil {
		t.Fatal(err)
	}
	r, err := Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	// A verified completed GitHub branch does not require the token again.
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if len(r.State.Commands) != before || len(r.State.Data.GitHub) != 1 {
		t.Fatal("completed GitHub branch repeated or lost")
	}
	// Corrupt a web checkpoint: rerun the web stages, keep GitHub's result.
	if err = os.WriteFile(filepath.Join(e.Dir, "raw", "katana.jsonl"), []byte("corrupt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err = Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	ghCalls := 0
	for _, c := range r.State.Commands {
		if c.Tool == "ghrecon" && c.Purpose == "scan" {
			ghCalls++
		}
	}
	if ghCalls != 1 || len(r.State.Data.GitHub) != 1 {
		t.Fatal("web retry repeated GitHub", ghCalls)
	}
}

func TestGitHubPartialAndRetry(t *testing.T) {
	e := githubFixture(t)
	t.Setenv("MOCK_FAIL_TOOL", "ghrecon")
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.State.Stages[1].Status != "Partial" || e.Meta.State != "Partial" || len(e.State.Data.GitHub) != 1 || e.State.Stages[2].Status != "Completed" {
		t.Fatal(e.State.Stages)
	}
	t.Setenv("MOCK_FAIL_TOOL", "")
	r, err := Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	r.GitHubToken = e.GitHubToken
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if r.Meta.State != "Completed" {
		t.Fatal(r.State.Stages)
	}
	for _, c := range r.State.Commands[len(e.State.Commands):] {
		if c.Tool != "ghrecon" {
			t.Fatal("GitHub retry repeated web stage", c.Tool)
		}
	}
}

type cancelGitHubExecutor struct{ started chan struct{} }

func (x cancelGitHubExecutor) Run(ctx context.Context, req runner.Request) error {
	if strings.Contains(filepath.Base(req.Executable), "ghrecon") && strings.Contains(strings.Join(req.Args, " "), "--jsonl") {
		close(x.started)
		<-ctx.Done()
		return ctx.Err()
	}
	return (runner.OSExecutor{}).Run(ctx, req)
}
func TestGitHubCancellationJoinsWorker(t *testing.T) {
	e := githubFixture(t)
	started := make(chan struct{})
	e.Executor = cancelGitHubExecutor{started: started}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-ctx.Done():
		}
	}()
	_ = e.Run(ctx, false)
	select {
	case <-e.githubBranch.done:
	default:
		t.Fatal("GitHub child survived workflow shutdown")
	}
	if e.Meta.State != "Interrupted" {
		t.Fatal(e.Meta.State)
	}
}

func TestGitHubStageTimeoutKeepsCompletedWebResults(t *testing.T) {
	e := githubFixture(t)
	e.Config.StageTimeout = 4 * time.Second
	e.Meta.Config = e.Config
	e.Executor = cancelGitHubExecutor{started: make(chan struct{})}
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.Meta.State != "Partial" || e.State.Stages[1].Status != "Interrupted" || e.State.Stages[2].Status != "Completed" {
		t.Fatal(e.Meta.State, e.State.Stages)
	}
}

func TestGitHubGlobalDeadlineWhileWaitingIsAnError(t *testing.T) {
	e := githubFixture(t)
	e.Config.Crawl = "off"
	e.Config.Directory = "off"
	e.Config.MaxDuration = 2 * time.Second
	e.Meta.Config = e.Config
	e.State.Stages = nil
	for _, stage := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, model.StageState{Name: stage.Name, Status: "Pending"})
	}
	e.Executor = cancelGitHubExecutor{started: make(chan struct{})}
	err := e.Run(context.Background(), false)
	if !errors.Is(err, context.DeadlineExceeded) || e.Meta.State != "Interrupted" {
		t.Fatal("global timeout reported success", err, e.Meta.State)
	}
}

func TestGitHubAutoMode(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("DISCOVERYX_GITHUB_TOKEN", "")
	for _, token := range []string{"", "test-token"} {
		c := config.Default()
		c.Target = "fixture.test"
		c.Output = t.TempDir()
		c.GitHubToken = token
		e := New(c, io.Discard)
		if err := e.Initialize(); err != nil {
			t.Fatal(err)
		}
		want := "off"
		if token != "" {
			want = "on"
		}
		if e.Config.GHRecon != want {
			t.Fatal(e.Config.GHRecon)
		}
	}
}
