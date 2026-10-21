package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubCredentialsStayOutOfMetadata(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token-only")
	c := Default()
	token, err := c.ResolveGitHubToken()
	if err != nil || token != "env-token-only" {
		t.Fatal(err)
	}
	c.GitHubToken = "github_pat_TEST_ONLY_SECRET"
	b, err := json.Marshal(c)
	if err != nil || strings.Contains(string(b), c.GitHubToken) {
		t.Fatal("token leaked into saved configuration")
	}
	for _, args := range [][]string{{"--github-token", c.GitHubToken}, {"--github-token=" + c.GitHubToken}} {
		if strings.Contains(strings.Join(RedactGitHubArgs(args), " "), c.GitHubToken) {
			t.Fatal("token in invocation")
		}
	}
	c.GitHubToken = ""
	c.GitHubTokenFile = filepath.Join(t.TempDir(), "토큰 파일.txt")
	if err = os.WriteFile(c.GitHubTokenFile, []byte("file-token-only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if token, err = c.ResolveGitHubToken(); err != nil || token != "file-token-only" {
		t.Fatal(err)
	}
	c.GitHubToken = "cli-token-only"
	if _, err = c.ResolveGitHubToken(); err == nil {
		t.Fatal("ambiguous token source accepted")
	}
}
