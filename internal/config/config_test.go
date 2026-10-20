package config

import (
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
