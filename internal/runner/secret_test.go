package runner

import (
	"bytes"
	"testing"
)

func TestSecretWriterAcrossEveryBoundary(t *testing.T) {
	secret := "github_pat_TEST_CREDENTIAL_ONLY"
	text := "before " + secret + " after " + secret + " end"
	for split := 0; split <= len(text); split++ {
		var output bytes.Buffer
		w := &SecretWriter{Writer: &output, Secret: secret}
		if _, err := w.Write([]byte(text[:split])); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(text[split:])); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		if output.String() != "before [REDACTED] after [REDACTED] end" {
			t.Fatalf("split %d: %s", split, output.String())
		}
	}
}
