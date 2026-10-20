package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvironmentPrecedence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte("target: fixture.test\nmax_targets: 4\nhttp_timeout: 3s\n"), 0600)
	t.Setenv("DISCOVERYX_MAX_TARGETS", "7")
	t.Setenv("DISCOVERYX_HTTP_TIMEOUT", "2s")
	c, e := Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if c.MaxTargets != 7 || c.HTTPTimeout != 2*time.Second {
		t.Fatal(c)
	}
	if e = c.Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestValidation(t *testing.T) {
	c := Default()
	c.Target = "fixture.test"
	c.Limits["katana"] = Limit{}
	if c.Validate() == nil {
		t.Fatal("unlimited default accepted")
	}
}

func TestNativeScannerLimits(t *testing.T) {
	c := Default()
	c.Target = "fixture.test"
	if !c.TemplateAutoInstall || c.Validate() != nil {
		t.Fatal("native defaults rejected")
	}
	for _, name := range []string{"feroxbuster", "nuclei"} {
		c.Limits[name] = Limit{Rate: 1000, Concurrency: 200}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
		c.Limits[name] = Limit{Rate: -1, Concurrency: 0}
		if c.Validate() == nil {
			t.Fatal("negative limit accepted")
		}
		c.Limits[name] = Limit{}
	}
	c.FeroxScanLimit = -1
	if c.Validate() == nil {
		t.Fatal("negative directory concurrency accepted")
	}
}

func TestBundledDefaultsOutsideSourceCheckout(t *testing.T) {
	t.Chdir(t.TempDir())
	c, e := Load("")
	if e != nil {
		t.Fatal(e)
	}
	if c.TechRules != BuiltinTechRules || c.Limits["feroxbuster"].Rate != 0 {
		t.Fatal(c)
	}
	if e = c.Absolute(); e != nil {
		t.Fatal(e)
	}
	if c.TechRules != BuiltinTechRules {
		t.Fatal("builtin rules converted into a file path")
	}
	b, e := ReadTechRules(c.TechRules)
	if e != nil || !bytes.Contains(b, []byte("extensions: [php]")) {
		t.Fatalf("%s %v", b, e)
	}
	if _, e = ReadTechRules("missing-custom-rules.yaml"); !os.IsNotExist(e) {
		t.Fatalf("custom missing file silently replaced: %v", e)
	}
}
