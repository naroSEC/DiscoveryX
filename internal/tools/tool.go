package tools

import (
	"context"
	"discoveryx/internal/config"
	"discoveryx/internal/model"
	"discoveryx/internal/runner"
	"discoveryx/internal/scope"
	"discoveryx/internal/storage"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Invocation struct {
	Args   []string
	Output string
}
type Input struct {
	PortHosts       map[string]string
	Help            string
	Config          config.Config
	Scope           scope.Validator
	RunDir          string
	HostsFile       string
	URLsFile        string
	Hosts           []string
	Services        []model.WebService
	Phase           string
	TemplatePaths   []string
	TemplateVersion string
	Warn            io.Writer
}
type Tool interface {
	Name() string
	Version(context.Context, runner.Executor, string) (string, error)
	Validate(context.Context, runner.Executor, string) error
	BuildArgs(Input) ([]Invocation, error)
	Run(context.Context, runner.Executor, string, Invocation, Input) error
	Parse(io.Reader, *model.Data, Input) []string
	RequiredInputs() []string
	Outputs() []string
}
type Adapter struct {
	name   string
	build  func(Input) ([]Invocation, error)
	parse  func(io.Reader, *model.Data, Input) []string
	inputs []string
}

func (a Adapter) Name() string             { return a.name }
func (a Adapter) RequiredInputs() []string { return a.inputs }
func (a Adapter) Outputs() []string        { return []string{a.name + ".jsonl"} }
func (a Adapter) Version(ctx context.Context, e runner.Executor, p string) (string, error) {
	f := "-version"
	if a.name == "feroxbuster" {
		f = "--version"
	}
	if a.name == "feroxbuster" {
		return runner.Probe(ctx, e, p, f)
	}
	return runner.Capture(ctx, e, p, f, "-duc")
}
func (a Adapter) Validate(ctx context.Context, e runner.Executor, p string) error {
	f := "-h"
	if a.name == "feroxbuster" {
		f = "--help"
	}
	s, err := runner.Probe(ctx, e, p, f)
	if err != nil {
		return err
	}
	if a.name != "feroxbuster" && !Supports(s, "-duc") {
		return fmt.Errorf("%s lacks -duc; cannot prevent automatic update checks", a.name)
	}
	return nil
}
func (a Adapter) BuildArgs(in Input) ([]Invocation, error)            { return a.build(in) }
func (a Adapter) Parse(r io.Reader, d *model.Data, in Input) []string { return a.parse(r, d, in) }
func (a Adapter) Run(ctx context.Context, e runner.Executor, p string, inv Invocation, in Input) error {
	hf := "-h"
	if a.name == "feroxbuster" {
		hf = "--help"
	}
	help, err := runner.Probe(ctx, e, p, hf)
	if err != nil {
		return err
	}
	for _, s := range inv.Args {
		if strings.HasPrefix(s, "-") && !Supports(help, s) {
			return fmt.Errorf("%s installed binary does not support required flag %s", a.name, s)
		}
	}
	if err = os.MkdirAll(filepath.Dir(inv.Output), 0700); err != nil {
		return err
	}
	if _, err = os.Stat(inv.Output); err == nil {
		return fmt.Errorf("refusing to overwrite raw result %s", inv.Output)
	}
	out, err := os.OpenFile(inv.Output+".stdout.log", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	log, err := os.OpenFile(inv.Output+".stderr.log", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	// Empty config files and private working directories prevent scanner defaults
	// from enabling redirects, uploads, headers or extra protocols.
	work := filepath.Join(in.RunDir, "sandbox", a.name)
	if err = os.MkdirAll(work, 0700); err != nil {
		return err
	}
	env := runner.Environment()
	if a.name == "feroxbuster" {
		userConfig, _ := os.UserConfigDir()
		for _, cp := range []string{filepath.Join(userConfig, "feroxbuster", "ferox-config.toml"), filepath.Join(filepath.Dir(p), "ferox-config.toml"), filepath.FromSlash("/etc/feroxbuster/ferox-config.toml")} {
			if _, ce := os.Stat(cp); ce == nil {
				return fmt.Errorf("external feroxbuster config could override scope/redirect policy: %s; move it before scanning", cp)
			}
		}
	}
	return e.Run(ctx, runner.Request{Executable: p, Args: inv.Args, Dir: work, Env: env, Stdout: out, Stderr: log})
}
func Supports(help, flag string) bool {
	return regexp.MustCompile(`(?m)(^|[\s,])` + regexp.QuoteMeta(flag) + `([\s,=]|$)`).MatchString(help)
}
func Registry() map[string]Tool {
	return map[string]Tool{"naabu": Naabu(), "subfinder": Subfinder(), "dnsx": DNSX(), "httpx": HTTPX(), "katana": Katana(), "feroxbuster": Feroxbuster(), "nuclei": Nuclei()}
}
func raw(in Input, name string) string { return filepath.Join(in.RunDir, "raw", name+".jsonl") }
func num(i int) string                 { return strconv.Itoa(i) }
func timeout(in Input) string {
	n := int(in.Config.HTTPTimeout.Seconds())
	if n < 1 {
		n = 1
	}
	return num(n)
}
func configArgs(in Input, name string) ([]string, error) {
	if name == "dnsx" {
		return []string{"-duc"}, nil
	} // dnsx 1.3.1 has no -config option.
	p := filepath.Join(in.RunDir, "inputs", name+"-config.yaml")
	if err := storage.Atomic(p, []byte("{}\n")); err != nil {
		return nil, err
	}
	return []string{"-config", p, "-duc"}, nil
}
func warn(in Input, s string) {
	if in.Warn != nil {
		fmt.Fprintln(in.Warn, s)
	}
}
