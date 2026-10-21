package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type Request struct {
	Executable     string
	Args           []string
	Dir            string
	Env            []string
	Stdout, Stderr io.Writer
}
type Executor interface {
	Run(context.Context, Request) error
}
type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, r Request) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	cmd := exec.CommandContext(ctx, r.Executable, r.Args...)
	cmd.Dir = r.Dir
	cmd.Env = r.Env
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.WaitDelay = 2 * time.Second
	attach, cleanup, e := prepare(cmd)
	if e != nil {
		return e
	}
	defer cleanup()
	if e = cmd.Start(); e != nil {
		return e
	}
	e = attach()
	if e != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return e
	}
	e = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return e
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 4<<20 {
		remain := (4 << 20) - b.Len()
		if len(p) > remain {
			p = p[:remain]
		}
		b.Buffer.Write(p)
	}
	return n, nil
}
func Capture(ctx context.Context, e Executor, path string, args ...string) (string, error) {
	var b limitedBuffer
	err := e.Run(ctx, Request{Executable: path, Args: args, Stdout: &b, Stderr: &b, Env: Environment()})
	return b.String(), err
}
func Environment() []string {
	env := []string{}
	for _, s := range os.Environ() { // Prevent inherited custom template download and proxy settings.
		if hasPrefixAny(s, "GITHUB_TOKEN=", "DISCOVERYX_GITHUB_TOKEN=", "NUCLEI_", "GITHUB_TEMPLATE_", "GITLAB_TEMPLATE_", "AWS_TEMPLATE_", "AZURE_TEMPLATE_", "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=") {
			continue
		}
		env = append(env, s)
	}
	return append(env, "DISABLE_NUCLEI_TEMPLATES_GITHUB_DOWNLOAD=true", "DISABLE_NUCLEI_TEMPLATES_GITLAB_DOWNLOAD=true", "DISABLE_NUCLEI_TEMPLATES_AWS_DOWNLOAD=true", "DISABLE_NUCLEI_TEMPLATES_AZURE_DOWNLOAD=true")
}
func hasPrefixAny(s string, p ...string) bool {
	for _, x := range p {
		if len(s) >= len(x) && equalFold(s[:len(x)], x) {
			return true
		}
	}
	return false
}
func equalFold(a, b string) bool {
	for i := range a {
		aa, bb := a[i], b[i]
		if aa >= 'a' && aa <= 'z' {
			aa -= 32
		}
		if bb >= 'a' && bb <= 'z' {
			bb -= 32
		}
		if aa != bb {
			return false
		}
	}
	return true
}
func Probe(ctx context.Context, e Executor, path, flag string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	s, err := Capture(ctx, e, path, flag)
	if err != nil {
		return s, fmt.Errorf("%s %s: %w", path, flag, err)
	}
	if s == "" {
		return s, errors.New("empty capability/version output")
	}
	return s, nil
}
