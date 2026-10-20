package cli

import (
	"bytes"
	"testing"
)

func TestMultiCharacterAliases(t *testing.T) {
	for _, mode := range []string{"first", "last", "both", "off"} {
		r, e := New([]string{"-u", "fixture.test", "-ps", mode, "-sbf", "off", "-cwl=off", "--naabu-rate", "7"}, &bytes.Buffer{}, &bytes.Buffer{})
		if e != nil {
			t.Fatal(e)
		}
		if e = r.ParseFlags(Normalize([]string{"-u", "fixture.test", "-ps", mode, "-sbf", "off", "-cwl=off", "--naabu-rate", "7"})); e != nil {
			t.Fatal(e)
		}
		for key, want := range map[string]string{"port-scan": mode, "subfinder": "off", "crawl": "off", "naabu-rate": "7"} {
			if got := r.Flags().Lookup(key).Value.String(); got != want {
				t.Fatalf("%s=%s want %s", key, got, want)
			}
		}
	}
}
func TestHelp(t *testing.T) {
	var out bytes.Buffer
	r, e := New([]string{"--help"}, &out, &out)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Execute(); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), []byte("web-only")) {
		t.Fatal(out.String())
	}
}
