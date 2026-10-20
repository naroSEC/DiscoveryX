package scope

import "testing"

func TestBoundaries(t *testing.T) {
	v, e := New("Example.com", []string{"private.example.com"})
	if e != nil {
		t.Fatal(e)
	}
	for input, want := range map[string]bool{"example.com": true, "api.example.com": true, "https://api.example.com/path": true, "example.com.attacker.test": false, "notexample.com": false, "private.example.com": false, "a.private.example.com": false, "https://user@example.com/": false, "https://example.com@evil.test/": false, "evil.test": false} {
		if got := v.Contains(input); got != want {
			t.Errorf("%s: %v", input, got)
		}
	}
}
func TestURL(t *testing.T) {
	a, e := URL("https://Example.com#x")
	if e != nil || a != "https://example.com/" {
		t.Fatalf("%s %v", a, e)
	}
}
