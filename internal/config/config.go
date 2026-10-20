package config

import (
	"discoveryx/internal/scope"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Limit struct {
	Rate        int `yaml:"rate"`
	Concurrency int `yaml:"concurrency"`
}
type Config struct {
	Target         string           `yaml:"target"`
	Output         string           `yaml:"output"`
	ToolsDir       string           `yaml:"tools_dir"`
	TemplatesDir   string           `yaml:"templates_dir"`
	PortScan       string           `yaml:"port_scan"`
	Subfinder      string           `yaml:"subfinder"`
	Crawl          string           `yaml:"crawl"`
	Directory      string           `yaml:"directory"`
	Nuclei         string           `yaml:"nuclei"`
	WebOnly        bool             `yaml:"web_only"`
	ScanType       string           `yaml:"scan_type"`
	SYNFallback    bool             `yaml:"syn_fallback"`
	SmartScan      bool             `yaml:"smart_scan"`
	ServiceVersion bool             `yaml:"service_version"`
	Probes         string           `yaml:"probes"`
	Ports          string           `yaml:"ports"`
	Wordlist       string           `yaml:"wordlist"`
	TechRules      string           `yaml:"tech_rules"`
	Extensions     []string         `yaml:"extensions"`
	CrawlDepth     int              `yaml:"crawl_depth"`
	DirectoryDepth int              `yaml:"directory_depth"`
	Tags           string           `yaml:"tags"`
	Severity       string           `yaml:"severity"`
	Approved       []string         `yaml:"approved"`
	Excluded       []string         `yaml:"excluded"`
	ApprovedIPs    []string         `yaml:"approved_ips"`
	HTTPTimeout    time.Duration    `yaml:"http_timeout"`
	StageTimeout   time.Duration    `yaml:"stage_timeout"`
	MaxDuration    time.Duration    `yaml:"max_duration"`
	MaxTargets     int              `yaml:"max_targets"`
	Limits         map[string]Limit `yaml:"limits"`
	LogLevel       string           `yaml:"log_level"`
}

func Default() Config {
	base, _ := os.UserCacheDir()
	base = filepath.Join(base, "discoveryx")
	c := Config{Output: "runs", ToolsDir: filepath.Join(base, "tools"), TemplatesDir: filepath.Join(base, "templates"), PortScan: "first", Subfinder: "on", Crawl: "on", Directory: "on", Nuclei: "on", ScanType: "connect", ServiceVersion: true, Ports: "top-1000", Wordlist: "wordlists/common.txt", TechRules: "configs/tech-extensions.yaml", CrawlDepth: 3, DirectoryDepth: 3, Tags: "cve,vuln,vkev,kev", Severity: "medium,high,critical", HTTPTimeout: 10 * time.Second, StageTimeout: 30 * time.Minute, MaxDuration: 2 * time.Hour, MaxTargets: 100, LogLevel: "info", Limits: map[string]Limit{}}
	for _, n := range []string{"naabu", "subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei"} {
		c.Limits[n] = Limit{Rate: 20, Concurrency: 5}
	}
	return c
}
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, e
		}
		d := yaml.NewDecoder(strings.NewReader(string(b)))
		d.KnownFields(true)
		if e = d.Decode(&c); e != nil {
			return c, e
		}
	}
	v := reflect.ValueOf(&c).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		key := "DISCOVERYX_" + strings.ToUpper(t.Field(i).Tag.Get("yaml"))
		if s, ok := os.LookupEnv(key); ok {
			field := v.Field(i)
			dest := reflect.New(field.Type())
			if field.Kind() == reflect.String {
				dest.Elem().SetString(s)
			} else if e := yaml.Unmarshal([]byte(s), dest.Interface()); e != nil {
				return c, fmt.Errorf("%s: %w", key, e)
			}
			field.Set(dest.Elem())
		}
	}
	return c, nil
}
func (c Config) Validate() error {
	if _, e := scope.New(c.Target, c.Approved, c.Excluded, c.ApprovedIPs); e != nil {
		return e
	}
	if !one(c.PortScan, "first", "last", "both", "off") {
		return fmt.Errorf("port_scan must be first/last/both/off")
	}
	for _, s := range []string{c.Subfinder, c.Crawl, c.Directory, c.Nuclei} {
		if !one(s, "on", "off") {
			return fmt.Errorf("workflow switches require on/off")
		}
	}
	if !one(c.ScanType, "connect", "syn") {
		return fmt.Errorf("scan_type must be connect/syn")
	}
	if c.HTTPTimeout <= 0 || c.StageTimeout <= 0 || c.MaxDuration <= 0 || c.MaxTargets < 1 || c.CrawlDepth < 1 || c.DirectoryDepth < 1 {
		return fmt.Errorf("timeouts, depths and maximum targets must be positive")
	}
	for _, n := range []string{"naabu", "subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei"} {
		l, ok := c.Limits[n]
		if !ok || l.Rate < 1 || l.Concurrency < 1 || l.Concurrency > 100 {
			return fmt.Errorf("%s needs positive rate/concurrency (concurrency <=100)", n)
		}
	}
	for _, s := range strings.Split(c.Severity, ",") {
		if !one(s, "info", "low", "medium", "high", "critical", "unknown") {
			return fmt.Errorf("invalid severity %s", s)
		}
	}
	if strings.TrimSpace(c.Tags) == "" {
		return fmt.Errorf("nuclei tags cannot be empty")
	}
	if c.Output == "" || c.ToolsDir == "" || c.TemplatesDir == "" {
		return fmt.Errorf("output/tools/templates paths cannot be empty")
	}
	if c.Ports == "" {
		return fmt.Errorf("ports cannot be empty")
	}
	if strings.HasPrefix(c.Ports, "top-") {
		if !one(c.Ports, "top-100", "top-1000", "top-full") {
			return fmt.Errorf("ports: supported presets are top-100/top-1000/top-full")
		}
	} else {
		for _, p := range strings.Split(c.Ports, ",") {
			bounds := strings.Split(p, "-")
			if len(bounds) > 2 {
				return fmt.Errorf("invalid TCP port range %s", p)
			}
			lo := 0
			for i, b := range bounds {
				v, e := strconv.Atoi(b)
				if e != nil || v < 1 || v > 65535 {
					return fmt.Errorf("invalid TCP port %s", b)
				}
				if i == 1 && v < lo {
					return fmt.Errorf("descending TCP port range %s", p)
				}
				lo = v
			}
		}
	}
	if !one(c.LogLevel, "info", "error", "debug") {
		return fmt.Errorf("log_level must be info/error/debug")
	}
	return nil
}
func one(s string, choices ...string) bool {
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}
func (c *Config) Absolute() error {
	for _, p := range []*string{&c.Output, &c.ToolsDir, &c.TemplatesDir, &c.Wordlist, &c.TechRules} {
		s, e := filepath.Abs(*p)
		if e != nil {
			return e
		}
		*p = s
	}
	if c.Probes != "" {
		s, e := filepath.Abs(c.Probes)
		if e != nil {
			return e
		}
		c.Probes = s
	}
	return nil
}
