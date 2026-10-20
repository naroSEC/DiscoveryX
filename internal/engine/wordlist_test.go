package engine

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/testutil"
)

func TestWordlistSelectionOutsideCheckout(t *testing.T) {
	exe := testutil.Binary(t)
	for _, mode := range []string{"builtin", "custom", "missing"} {
		t.Run(mode, func(t *testing.T) {
			e := webOnlyFixture(t, exe)
			e.Config.TechRules = config.BuiltinTechRules
			switch mode {
			case "builtin":
				e.Config.Wordlist = config.BuiltinWordlist
			case "missing":
				e.Config.Wordlist = filepath.Join(t.TempDir(), "missing.txt")
			}
			e.Meta.Config = e.Config
			t.Chdir(t.TempDir())
			if err := e.Run(context.Background(), false); err != nil {
				t.Fatal(err)
			}
			stage := e.State.Stages[1]
			if mode == "missing" {
				if stage.Status != "Skipped" || !strings.Contains(stage.Reason, "wordlist unavailable") {
					t.Fatal(stage)
				}
				return
			}
			if stage.Status != "Completed" {
				t.Fatal(stage)
			}
			want, err := config.ReadWordlist(e.Config.Wordlist)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := filepath.Join(e.Dir, "inputs", "wordlist.txt")
			got, err := os.ReadFile(snapshot)
			if err != nil || !bytes.Equal(want, got) {
				t.Fatal("wrong dictionary passed to scanner", err)
			}
			found := false
			for _, command := range e.State.Commands {
				if command.Tool != "feroxbuster" || command.Purpose != "scan" {
					continue
				}
				for i, arg := range command.Args {
					if arg == "-w" && i+1 < len(command.Args) && command.Args[i+1] == snapshot {
						found = true
					}
				}
			}
			if !found {
				t.Fatal("scanner did not use dictionary snapshot")
			}
			r, err := Resume(e.Dir, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if err = r.Run(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			if len(r.State.Commands) != len(e.State.Commands) {
				t.Fatal("completed dictionary scan rerun on resume")
			}
		})
	}
}
