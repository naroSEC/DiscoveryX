package cli

import (
	"context"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/engine"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/tools"
	"github.com/naroSEC/DiscoveryX/internal/updater"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var Version = "0.2.0"

func Normalize(args []string) []string {
	out := append([]string{}, args...)
	aliases := map[string]string{"-ps": "--port-scan", "-sbf": "--subfinder", "-cwl": "--crawl"}
	for i, s := range out {
		if s == "--" {
			break
		}
		for k, v := range aliases {
			if s == k || strings.HasPrefix(s, k+"=") {
				out[i] = v + strings.TrimPrefix(s, k)
			}
		}
	}
	return out
}
func New(args []string, out, errOut io.Writer) (*cobra.Command, error) {
	args = Normalize(args)
	path := ""
	for i, s := range args {
		if strings.HasPrefix(s, "--config=") {
			path = strings.TrimPrefix(s, "--config=")
		} else if s == "--config" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	if path == "" {
		for _, p := range []string{"config.yaml", "configs/config.yaml"} {
			if _, e := os.Stat(p); e == nil {
				path = p
				break
			}
		}
	}
	c, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	root := &cobra.Command{Use: "discoveryx", Short: "Sequential, scoped security reconnaissance using official scanners", Version: Version, SilenceUsage: true, SilenceErrors: true, RunE: func(cmd *cobra.Command, _ []string) error {
		if e := c.Validate(); e != nil {
			return e
		}
		if e := c.Absolute(); e != nil {
			return e
		}
		eng := engine.New(c, out)
		if e := eng.Initialize(); e != nil {
			return e
		}
		e := eng.Run(cmd.Context(), false)
		eng.PrintSummary()
		if e != nil {
			return e
		}
		if eng.Meta.State == "Partial" {
			return fmt.Errorf("run completed with failed/skipped stages; inspect summary.json")
		}
		return nil
	}, Args: cobra.NoArgs}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	f := root.PersistentFlags()
	bind(f, &c, &path)
	root.AddCommand(toolsCommand(&c, out), templatesCommand(&c, out), doctorCommand(&c, out), resumeCommand(out))
	return root, nil
}
func Execute(ctx context.Context, args []string, out, errOut io.Writer) error {
	root, err := New(args, out, errOut)
	if err != nil {
		return err
	}
	return root.ExecuteContext(ctx)
}
func bind(f *pflag.FlagSet, c *config.Config, path *string) {
	f.StringVar(path, "config", *path, "YAML configuration path (CLI > DISCOVERYX_* > YAML > defaults)")
	f.StringVarP(&c.Target, "url", "u", c.Target, "Target domain or HTTP URL")
	f.StringVarP(&c.Output, "output", "o", c.Output, "Output directory")
	f.StringVar(&c.ToolsDir, "tools-dir", c.ToolsDir, "Managed executable directory")
	f.StringVar(&c.TemplatesDir, "templates-dir", c.TemplatesDir, "Managed Nuclei template directory")
	f.StringVar(&c.PortScan, "port-scan", c.PortScan, "first/last/both/off (also -ps)")
	f.StringVar(&c.Subfinder, "subfinder", c.Subfinder, "on/off (also -sbf)")
	f.StringVar(&c.Crawl, "crawl", c.Crawl, "on/off (also -cwl)")
	f.StringVar(&c.Directory, "dir", c.Directory, "on/off")
	f.StringVar(&c.Nuclei, "nuc", c.Nuclei, "on/off")
	f.BoolVar(&c.WebOnly, "web-only", c.WebOnly, "Fingerprint the input URL with httpx, then run enabled web stages")
	f.StringVar(&c.ScanType, "scan-type", c.ScanType, "connect/syn")
	f.BoolVar(&c.SYNFallback, "syn-fallback", c.SYNFallback, "Explicitly allow Connect fallback if SYN is unavailable")
	f.BoolVar(&c.SmartScan, "smart-scan", c.SmartScan, "Enable Naabu Smart Scan (-ss)")
	f.BoolVar(&c.ServiceVersion, "service-version", c.ServiceVersion, "Enable service versions when local probes exist")
	f.StringVar(&c.Probes, "probes", c.Probes, "nmap-service-probes file")
	f.StringVar(&c.Ports, "ports", c.Ports, "top-1000 or explicit TCP ports/ranges")
	f.StringVar(&c.Wordlist, "wordlist", c.Wordlist, "User-provided directory wordlist")
	f.StringVar(&c.TechRules, "tech-rules", c.TechRules, "YAML technology-to-extension rules")
	f.StringSliceVar(&c.Extensions, "extensions", c.Extensions, "Additional directory extensions")
	f.IntVar(&c.CrawlDepth, "crawl-depth", c.CrawlDepth, "Crawl depth")
	f.IntVar(&c.DirectoryDepth, "dir-depth", c.DirectoryDepth, "Directory depth")
	f.StringVar(&c.Tags, "nuclei-tags", c.Tags, "Nuclei template tags")
	f.StringVar(&c.Severity, "nuclei-severity", c.Severity, "Nuclei severities")
	f.StringVar(&c.LogLevel, "log-level", c.LogLevel, "info/error/debug (full logs are always saved)")
	f.StringSliceVar(&c.Excluded, "exclude", c.Excluded, "Excluded hosts and subdomains")
	f.DurationVar(&c.HTTPTimeout, "http-timeout", c.HTTPTimeout, "HTTP request timeout")
	f.DurationVar(&c.StageTimeout, "stage-timeout", c.StageTimeout, "Maximum duration per stage")
	f.DurationVar(&c.MaxDuration, "max-duration", c.MaxDuration, "Maximum active duration across resumes")
	f.IntVar(&c.MaxTargets, "max-targets", c.MaxTargets, "Maximum input targets per stage")
	for _, name := range updater.Names {
		n := name
		l := c.Limits[n]
		f.IntVar(&l.Rate, n+"-rate", l.Rate, "Requests/packets per second for "+n)
		f.IntVar(&l.Concurrency, n+"-concurrency", l.Concurrency, "Bounded workers for "+n)
		f.Lookup(n + "-rate").Value = &limitValue{c: c, name: n, rate: true, fallback: l.Rate}
		f.Lookup(n + "-concurrency").Value = &limitValue{c: c, name: n, fallback: l.Concurrency}
	}
}

type limitValue struct {
	c        *config.Config
	name     string
	rate     bool
	fallback int
}

func (v *limitValue) String() string {
	l := v.c.Limits[v.name]
	if v.rate {
		return fmt.Sprint(l.Rate)
	}
	return fmt.Sprint(l.Concurrency)
}
func (v *limitValue) Type() string { return "int" }
func (v *limitValue) Set(s string) error {
	var n int
	n, e := strconv.Atoi(s)
	if e != nil {
		return e
	}
	l := v.c.Limits[v.name]
	if v.rate {
		l.Rate = n
	} else {
		l.Concurrency = n
	}
	if v.c.Limits == nil {
		v.c.Limits = map[string]config.Limit{}
	}
	v.c.Limits[v.name] = l
	return nil
}
func toolsCommand(c *config.Config, out io.Writer) *cobra.Command {
	parent := &cobra.Command{Use: "tools", Short: "Install, list or explicitly update official release executables"}
	var tool string
	parent.PersistentFlags().StringVar(&tool, "tool", "", "Only this tool")
	for _, action := range []string{"install", "update", "list"} {
		a := action
		parent.AddCommand(&cobra.Command{Use: a, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.Absolute(); err != nil {
				return err
			}
			names := updater.Names
			if tool != "" {
				if _, ok := updater.Repositories[tool]; !ok {
					return fmt.Errorf("unknown tool %s", tool)
				}
				names = []string{tool}
			}
			if a == "list" {
				m, err := updater.ReadManifest(c.ToolsDir)
				if err != nil {
					return err
				}
				for _, n := range names {
					entry, ok := m[n]
					if !ok {
						fmt.Fprintf(out, "%-12s %-12s Not installed\n", n, "-")
						continue
					}
					status := "Installed"
					h, e := storage.Hash(entry.Path)
					if e != nil || h != entry.SHA256 {
						status = "Integrity error"
					}
					fmt.Fprintf(out, "%-12s %-12s %s (%s)\n", n, entry.Version, status, entry.Verification)
				}
				return nil
			}
			unlock, err := storage.Lock(c.ToolsDir)
			if err != nil {
				return err
			}
			defer unlock()
			m := updater.New(c.ToolsDir, out)
			var failed []string
			for _, n := range names {
				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
				_, err = m.Ensure(ctx, n, a == "update")
				cancel()
				if err != nil {
					fmt.Fprintf(out, "%s: %v\n", n, err)
					failed = append(failed, n)
				}
			}
			if len(failed) > 0 {
				return fmt.Errorf("tools unavailable: %s", strings.Join(failed, ", "))
			}
			return nil
		}})
	}
	return parent
}
func templatesCommand(c *config.Config, out io.Writer) *cobra.Command {
	parent := &cobra.Command{Use: "templates", Short: "Explicitly manage Nuclei templates independently of engine updates"}
	parent.AddCommand(&cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var state tools.TemplateState
		if err := storage.ReadJSON(filepath.Join(c.TemplatesDir, "discoveryx-templates.json"), &state); err != nil {
			return fmt.Errorf("templates not managed/installed: %w", err)
		}
		hash, count, err := tools.Tree(c.TemplatesDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\nFiles: %d | SHA256: %s | Updated: %s | Integrity: %t\n", state.VersionOutput, count, hash, state.Updated.Format(time.RFC3339), hash == state.TreeSHA256)
		if hash != state.TreeSHA256 {
			return fmt.Errorf("template tree changed")
		}
		return nil
	}})
	parent.AddCommand(&cobra.Command{Use: "update", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := c.Absolute(); err != nil {
			return err
		}
		unlock, err := storage.Lock(c.ToolsDir)
		if err != nil {
			return err
		}
		defer unlock()
		unlockT, err := storage.Lock(c.TemplatesDir + ".management")
		if err != nil {
			return err
		}
		defer unlockT()
		m := updater.New(c.ToolsDir, out)
		installation, err := m.Ensure(cmd.Context(), "nuclei", false)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
		defer cancel()
		s, err := tools.UpdateTemplates(ctx, runner.OSExecutor{}, installation.Path, c.TemplatesDir)
		if err == nil {
			fmt.Fprintf(out, "Templates: %s\nSHA256: %s\n", s.VersionOutput, s.TreeSHA256)
		}
		return err
	}})
	return parent
}
func doctorCommand(c *config.Config, out io.Writer) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check installed binaries, capabilities, wordlist, probes and SYN prerequisites without scanning", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		manifest, err := updater.ReadManifest(c.ToolsDir)
		if err != nil {
			return err
		}
		m := updater.New(c.ToolsDir, out)
		for _, n := range updater.Names {
			in, ok := manifest[n]
			if !ok {
				fmt.Fprintf(out, "%s: not installed (tools install --tool %s)\n", n, n)
				continue
			}
			v, e := m.Check(cmd.Context(), in.Path, n)
			if e == nil {
				e = tools.Registry()[n].Validate(cmd.Context(), runner.OSExecutor{}, in.Path)
			}
			fmt.Fprintf(out, "%s: version=%s check=%v\n", n, v, e)
		}
		_, wordlistErr := os.Stat(c.Wordlist)
		fmt.Fprintf(out, "Wordlist: %s (%v)\nService probes: %s\nSYN prerequisites: %v\n", c.Wordlist, wordlistErr, tools.FindProbes(c.Probes), tools.SYNAvailable())
		hash, count, err := tools.Tree(c.TemplatesDir)
		fmt.Fprintf(out, "Templates: files=%d sha256=%s check=%v\n", count, hash, err)
		return nil
	}}
}
func resumeCommand(out io.Writer) *cobra.Command {
	var run string
	c := &cobra.Command{Use: "resume", Short: "Resume a run directory using saved scope, settings, binary hashes and checkpoints", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		e, err := engine.Resume(run, out)
		if err != nil {
			return err
		}
		err = e.Run(cmd.Context(), true)
		e.PrintSummary()
		if err == nil && e.Meta.State == "Partial" {
			err = fmt.Errorf("run remains partial; inspect summary.json")
		}
		return err
	}}
	c.Flags().StringVar(&run, "run", "", "Existing run directory")
	_ = c.MarkFlagRequired("run")
	return c
}
