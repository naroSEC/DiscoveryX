package runner_test

import (
	"context"
	"errors"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"testing"
)

type recordStub struct {
	called  int
	err     error
	request runner.Request
}

func (s *recordStub) Run(_ context.Context, r runner.Request) error {
	s.called++
	s.request = r
	return s.err
}
func TestRecorderCapturesActualArgumentsAndFailure(t *testing.T) {
	stub := &recordStub{err: errors.New("fixture failure")}
	var events []model.Command
	rec := runner.Recorder{Executor: stub, Stage: "httpx", Tool: "httpx", Save: func(c model.Command) error { events = append(events, c); return nil }}
	args := []string{"-u", "https://fixture.test/path?x='a'&q=<script>", "-o", "C:\\결과 경로\\httpx.jsonl"}
	err := rec.Run(context.Background(), runner.Request{Executable: "C:\\도구 경로\\httpx.exe", Args: args})
	if err == nil || stub.called != 1 || len(events) != 2 {
		t.Fatalf("%v %d %v", err, stub.called, events)
	}
	if events[0].Status != "Running" || events[1].Status != "Failed" || events[1].Purpose != "scan" || events[1].Error != "fixture failure" || events[0].ID != events[1].ID {
		t.Fatal(events)
	}
	for i, s := range args {
		if stub.request.Args[i] != s || events[1].Args[i] != s {
			t.Fatal("recording changed arguments")
		}
	}
}
func TestRecorderDoesNotRunIfStartCannotBeSaved(t *testing.T) {
	stub := &recordStub{}
	rec := runner.Recorder{Executor: stub, Save: func(model.Command) error { return errors.New("disk full") }}
	if rec.Run(context.Background(), runner.Request{Executable: "not-executed"}) == nil || stub.called != 0 {
		t.Fatal("executed without a command record")
	}
}
