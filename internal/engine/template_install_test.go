package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naroSEC/DiscoveryX/internal/testutil"
)

func TestMissingTemplatesInstalledBeforeNuclei(t *testing.T) {
	e := fixture(t, testutil.Binary(t))
	if err := os.Remove(filepath.Join(e.Config.TemplatesDir, "fixture.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.Meta.State != "Completed" || len(e.State.Data.Vulnerabilities) == 0 {
		t.Fatalf("missing templates prevented scan: %s %+v", e.Meta.State, e.State.Stages)
	}
	updates := 0
	for _, command := range e.State.Commands {
		for _, arg := range command.Args {
			if arg == "-ut" {
				updates++
			}
		}
	}
	if updates != 1 {
		t.Fatalf("expected one initial template install, got %d", updates)
	}
	t.Setenv("MOCK_UPDATE_FAIL", "1")
	r, err := Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if r.Meta.State != "Completed" || len(r.State.Commands) != len(e.State.Commands) {
		t.Fatal("resume reinstalled templates or repeated completed stages")
	}
}

func TestMissingTemplateInstallFailuresAndOptOut(t *testing.T) {
	for _, auto := range []bool{true, false} {
		t.Run(map[bool]string{true: "download-failure", false: "installation-disabled"}[auto], func(t *testing.T) {
			e := fixture(t, testutil.Binary(t))
			e.Config.TemplateAutoInstall = auto
			e.Meta.Config.TemplateAutoInstall = auto
			if err := os.Remove(filepath.Join(e.Config.TemplatesDir, "fixture.yaml")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MOCK_UPDATE_FAIL", "1")
			if err := e.Run(context.Background(), false); err != nil {
				t.Fatal(err)
			}
			for _, stage := range e.State.Stages {
				if stage.Name != "nuclei" {
					continue
				}
				want := "Skipped"
				if auto {
					want = "Failed"
				}
				if stage.Status != want || !strings.Contains(stage.Reason, "discoveryx templates update") {
					t.Fatalf("%s %s", stage.Status, stage.Reason)
				}
			}
			if len(e.State.Data.Vulnerabilities) != 0 || e.Meta.State != "Partial" {
				t.Fatal("unexecuted checks reported as success")
			}
			if !auto {
				for _, command := range e.State.Commands {
					for _, arg := range command.Args {
						if arg == "-ut" {
							t.Fatal("opt-out still installed templates")
						}
					}
				}
			}
		})
	}
}
