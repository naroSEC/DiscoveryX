// mockscanner is an offline scanner fixture. It never makes network requests.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func arg(k string) string {
	for i, a := range os.Args[1:] {
		if a == k && i+2 < len(os.Args) {
			return os.Args[i+2]
		}
	}
	return ""
}
func has(k string) bool {
	for _, a := range os.Args[1:] {
		if a == k {
			return true
		}
	}
	return false
}
func main() {
	if has("--help-extra") {
		return
	}
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if has("-h") || has("--help") {
		fmt.Println(" -h --help -version --version -config -duc -d -silent -nc -rl -o -l -a -aaaa -cname -j -t -retry -sc -title -td -ip -cdn -server -timeout -nfs -u -jc -kf -fs -cs -cos -dr -do -c -p --depth -w -s --rate-limit --scan-limit --timeout --no-state --json --dont-scan --dont-extract-links -x -list -rate -top-ports -ss -sV -sV-probes -ud -tags -pt -etags -ni -dut -bs -tl -ut -tv")
		return
	}
	if has("-version") || has("--version") {
		v := os.Getenv("MOCK_VERSION")
		if v == "" {
			v = "1.2.3"
		}
		fmt.Println(name, v)
		return
	}
	if has("--sleep") {
		time.Sleep(10 * time.Minute)
		return
	}
	if has("--child") {
		cmd := exec.Command(os.Args[0], "--sleep")
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		_ = os.WriteFile(arg("--child"), []byte(fmt.Sprint(cmd.Process.Pid)), 0600)
		time.Sleep(10 * time.Minute)
		return
	}
	if has("--echo") {
		fmt.Print(arg("--echo"))
		return
	}
	if log := os.Getenv("MOCK_RECORD"); log != "" {
		f, _ := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if f != nil {
			fmt.Fprintln(f, name, strings.Join(os.Args[1:], " "))
			f.Close()
		}
	}
	if os.Getenv("MOCK_SLEEP_TOOL") == name {
		time.Sleep(10 * time.Minute)
	}
	if has("-ut") {
		if os.Getenv("MOCK_UPDATE_FAIL") != "" {
			os.Exit(2)
		}
		dir := arg("-ud")
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "fixture.yaml"), []byte("id: fixture\ninfo:\n  name: fixture\n  severity: high\n  tags: cve,kev\nhttp:\n  - method: GET\n    path: ['{{BaseURL}}/test']\n    matchers:\n      - type: status\n        status: [200]\n"), 0600)
		return
	}
	if has("-tv") {
		fmt.Println("nuclei-templates mock-1.0.0")
		return
	}
	if has("-tl") {
		b, _ := os.ReadFile(arg("-t"))
		fmt.Print(string(b))
		return
	}
	out := arg("-o")
	if out == "" {
		fmt.Fprintln(os.Stderr, "missing -o")
		os.Exit(3)
	}
	var rows []map[string]any
	var text string
	switch name {
	case "subfinder":
		text = "api.fixture.test\napi.fixture.test\nfixture.test.attacker.test\n"
	case "dnsx":
		b, _ := os.ReadFile(arg("-l"))
		for _, h := range strings.Fields(string(b)) {
			rows = append(rows, map[string]any{"host": h, "a": []string{"127.0.0.1"}, "status_code": "NOERROR"})
		}
	case "httpx":
		b, _ := os.ReadFile(arg("-l"))
		for _, h := range strings.Fields(string(b)) {
			u := h
			if !strings.Contains(h, "://") {
				u = "https://" + h
			}
			rows = append(rows, map[string]any{"url": u, "host": "127.0.0.1", "status_code": 200, "title": "fixture", "tech": []string{"PHP"}})
		}
	case "katana":
		rows = []map[string]any{{"request": map[string]any{"endpoint": "https://api.fixture.test/path?a=1", "source": "https://api.fixture.test/", "method": "GET"}}, {"request": map[string]any{"endpoint": "https://outside.test/"}}}
	case "feroxbuster":
		rows = []map[string]any{{"type": "response", "url": strings.TrimRight(arg("-u"), "/") + "/admin", "status": 200}}
	case "nuclei":
		if os.Getenv("MOCK_ZERO_TEMPLATES") == "1" {
			fmt.Fprintln(os.Stderr, "Templates loaded for current scan: 0")
			break
		}
		fmt.Fprintln(os.Stderr, "Templates loaded for current scan: 1")
		row := map[string]any{"template-id": "fixture", "matched-at": "https://fixture.test/test", "host": "https://fixture.test/", "matcher-name": "status", "timestamp": "2026-01-01T00:00:00Z", "info": map[string]any{"name": "fixture result", "severity": "high", "classification": map[string]any{"cve-id": []string{"CVE-2026-0000"}}}}
		rows = []map[string]any{row, row}
	case "naabu":
		b, _ := os.ReadFile(arg("-list"))
		for _, ip := range strings.Fields(string(b)) {
			rows = append(rows, map[string]any{"ip": ip, "port": 8080, "protocol": "tcp", "service": "http"})
		}
	}
	for _, r := range rows {
		b, _ := json.Marshal(r)
		text += string(b) + "\n"
	}
	if os.Getenv("MOCK_EMPTY_TOOL") == name {
		text = ""
	}
	if os.Getenv("MOCK_MALFORMED") == name {
		text += "{broken\n"
	}
	if err := os.WriteFile(out, []byte(text), 0600); err != nil {
		panic(err)
	}
	if os.Getenv("MOCK_FAIL_TOOL") == name {
		fmt.Fprintln(os.Stderr, "mock scanner failure")
		os.Exit(2)
	}
}
