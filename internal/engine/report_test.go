package engine

import (
	"context"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/reporting"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/testutil"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTMLReportAndCommandHistory(t *testing.T) {
	e := webOnlyFixture(t, testutil.Binary(t))
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(e.State.Commands) == 0 {
		t.Fatal("no command history")
	}
	var history []model.Command
	if err := storage.ReadJSON(filepath.Join(e.Dir, "logs", "commands.json"), &history); err != nil {
		t.Fatal(err)
	}
	scans := 0
	for _, command := range history {
		if command.Purpose == "scan" {
			scans++
			if command.Status != "Completed" || command.Executable == "" || len(command.Args) == 0 || command.Stage == "" || command.Stdout == "" || command.Stderr == "" {
				t.Fatal(command)
			}
		}
	}
	if scans != 2 {
		t.Fatalf("expected httpx and feroxbuster calls, got %d", scans)
	}
	b, err := os.ReadFile(filepath.Join(e.Dir, "report.html"))
	if err != nil || !strings.Contains(string(b), "application/json") {
		t.Fatal("HTML report not generated", err)
	}
	r, err := Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if len(r.State.Commands) != len(history) {
		t.Fatal("resume duplicated command history")
	}
	snapshot, err := reporting.Read(e.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Commands) != len(history) || snapshot.Target != e.Config.Target {
		t.Fatal("saved report fields differ")
	}
}

func TestRetryKeepsHistoricalCommandLogReferences(t *testing.T) {
	e := &Engine{Dir: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(e.Dir, "raw"), 0700); err != nil {
		t.Fatal(err)
	}
	log := "raw/httpx.jsonl.stdout.log"
	e.State.Commands = []model.Command{{ID: "old", Stdout: log}}
	if err := os.WriteFile(filepath.Join(e.Dir, filepath.FromSlash(log)), []byte("first attempt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.archivePrevious(filepath.Join(e.Dir, "raw", "httpx.jsonl")); err != nil {
		t.Fatal(err)
	}
	ref := e.State.Commands[0].Stdout
	if !strings.HasPrefix(ref, log+".previous-") {
		t.Fatal("historical log still points at retry output", ref)
	}
	b, err := os.ReadFile(filepath.Join(e.Dir, filepath.FromSlash(ref)))
	if err != nil || string(b) != "first attempt" {
		t.Fatal("historical log lost", err)
	}
}
