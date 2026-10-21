package config

import (
	"fmt"
	"os"
	"strings"
)

// GitHubToken resolves runtime credentials. Neither values nor token file paths
// are serialized into run configuration.
func (c Config) ResolveGitHubToken() (string, error) {
	if c.GitHubToken != "" && c.GitHubTokenFile != "" {
		return "", fmt.Errorf("use only one of --github-token and --github-token-file")
	}
	if c.GitHubToken != "" {
		return strings.TrimSpace(c.GitHubToken), nil
	}
	if c.GitHubTokenFile != "" {
		b, err := os.ReadFile(c.GitHubTokenFile)
		if err != nil {
			return "", fmt.Errorf("read GitHub token file: %w", err)
		}
		token := strings.TrimSpace(string(b))
		if token == "" || strings.ContainsAny(token, "\r\n\x00") {
			return "", fmt.Errorf("GitHub token file must contain one nonempty token")
		}
		return token, nil
	}
	if token := strings.TrimSpace(os.Getenv("DISCOVERYX_GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN")), nil
}

// RedactGitHubArgs preserves the command structure while removing credentials.
func RedactGitHubArgs(args []string) []string {
	out := append([]string{}, args...)
	for i, arg := range out {
		if arg == "--github-token" && i+1 < len(out) {
			out[i+1] = "[REDACTED]"
		}
		if strings.HasPrefix(arg, "--github-token=") {
			out[i] = "--github-token=[REDACTED]"
		}
	}
	return out
}
