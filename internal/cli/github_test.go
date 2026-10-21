package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naroSEC/DiscoveryX/internal/testutil"
)

func TestGitHubCLIRecordsRedactedInvocation(t *testing.T) {
	dir := t.TempDir()
	tools := filepath.Join(dir, "tools")
	testutil.Install(t, testutil.Binary(t), tools)
	const token = "github_pat_CLI_OFFLINE_TEST_ONLY"
	args := []string{"-u", "https://www.example.com", "--web-only", "--crawl", "off", "--dir", "off", "--nuc", "off", "--tools-dir", tools, "--output", filepath.Join(dir, "runs"), "--github-token", token}
	var output bytes.Buffer
	if err := Execute(context.Background(), args, &output, &output); err != nil {
		t.Fatal(err, output.String())
	}
	if strings.Contains(output.String(), token) {
		t.Fatal("console exposed GitHub token")
	}
	found := false
	if err := filepath.WalkDir(filepath.Join(dir, "runs"), func(path string, entry os.DirEntry, err error) error {
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
		if strings.Contains(string(b), token) {
			t.Fatalf("token persisted to %s", path)
		}
		if entry.Name() == "metadata.json" && strings.Contains(string(b), "[REDACTED]") {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("redacted original CLI command missing")
	}
}
