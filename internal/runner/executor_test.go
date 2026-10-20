package runner_test

import (
	"bytes"
	"context"
	"discoveryx/internal/runner"
	"discoveryx/internal/testutil"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRealProcessPathsTimeoutAndTree(t *testing.T) {
	exe := testutil.Binary(t)
	e := runner.OSExecutor{}
	var out bytes.Buffer
	literal := "한글 공백 & echo NOT_A_SHELL"
	if err := e.Run(context.Background(), runner.Request{Executable: exe, Args: []string{"--echo", literal}, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != literal {
		t.Fatal(out.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := e.Run(ctx, runner.Request{Executable: exe, Args: []string{"--sleep"}})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("%v %s", err, time.Since(start))
	}
	pidFile := t.TempDir() + "/child.pid"
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()
	err = e.Run(ctx2, runner.Request{Executable: exe, Args: []string{"--child", pidFile}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if processAlive(pid) {
		t.Fatalf("descendant process %d survived cancellation", pid)
	}
}
