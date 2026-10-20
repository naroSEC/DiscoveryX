package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportCommandUsesSavedRun(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "한글 결과")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"metadata.json": `{"config":{"target":"fixture.test"},"state":"Completed"}`,
		"status.json":   `{"data":{},"stages":[]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	cmd, err := New([]string{"report", "--run", dir}, &output, &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), filepath.Join(dir, "report.html")) {
		t.Fatal(output.String())
	}
	if _, err = os.Stat(filepath.Join(dir, "report.html")); err != nil {
		t.Fatal(err)
	}
}

func TestReportRequiresRunDirectory(t *testing.T) {
	cmd, err := New([]string{"report"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Execute(); err == nil {
		t.Fatal("report accepted a missing run directory")
	}
}
