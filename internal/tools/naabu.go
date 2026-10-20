package tools

import (
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/parser"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func FindProbes(custom string) string {
	if custom != "" {
		if st, e := os.Stat(custom); e == nil && !st.IsDir() {
			return custom
		}
		return ""
	}
	paths := []string{custom, filepath.Join(os.Getenv("ProgramFiles"), "Nmap", "nmap-service-probes"), filepath.Join(os.Getenv("ProgramFiles(x86)"), "Nmap", "nmap-service-probes"), "/usr/share/nmap/nmap-service-probes", "/usr/local/share/nmap/nmap-service-probes"}
	for _, p := range paths {
		if p != "" {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}
func Naabu() Tool {
	return Adapter{name: "naabu", inputs: []string{"port_hosts"}, build: func(in Input) ([]Invocation, error) {
		scan := "c"
		if in.Config.ScanType == "syn" {
			if e := SYNAvailable(); e != nil {
				if !in.Config.SYNFallback {
					return nil, e
				}
				warn(in, "SYN unavailable: "+e.Error()+"; explicit syn_fallback enabled, using TCP Connect")
			} else {
				scan = "s"
			}
		}
		p := raw(in, "naabu_"+in.Phase)
		a, e := configArgs(in, "naabu")
		if e != nil {
			return nil, e
		}
		l := in.Config.Limits["naabu"]
		a = append(a, "-list", in.HostsFile, "-s", scan, "-rate", num(l.Rate), "-c", num(l.Concurrency), "-j", "-o", p)
		if strings.HasPrefix(in.Config.Ports, "top-") {
			a = append(a, "-top-ports", strings.TrimPrefix(in.Config.Ports, "top-"))
		} else {
			a = append(a, "-p", in.Config.Ports)
		}
		if in.Config.SmartScan {
			a = append(a, "-ss")
		}
		if in.Config.ServiceVersion {
			probes := FindProbes(in.Config.Probes)
			if probes == "" {
				warn(in, "nmap-service-probes missing; service version detection disabled, port scan continues")
			} else if !Supports(in.Help, "-sV-probes") {
				warn(in, "installed Naabu lacks -sV-probes; native version detection cannot be configured safely; regular port scan continues")
			} else {
				a = append(a, "-sV", "-sV-probes", probes)
			}
		}
		return []Invocation{{a, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, _ int) {
			h := parser.Str(m, "host")
			ip := parser.Str(m, "ip")
			if _, ok := in.PortHosts[ip]; !ok || !in.Scope.Contains(in.PortHosts[ip]) {
				d.External = append(d.External, ip)
				return
			}
			if h == "" {
				h = ip
			}
			if mapped := in.PortHosts[ip]; mapped != "" {
				h = mapped
			}
			svc := parser.Str(m, "service")
			version := parser.Str(m, "version")
			if sm := parser.Map(m, "service"); len(sm) > 0 {
				svc = parser.Str(sm, "name")
				version = parser.Str(sm, "version")
			}
			proto := parser.Str(m, "protocol")
			if proto == "" {
				proto = "tcp"
			}
			port := parser.Int(m, "port")
			if port < 1 || port > 65535 {
				return
			}
			d.Ports = append(d.Ports, model.Port{Host: h, IP: ip, Port: port, Protocol: proto, Service: svc, Version: version, Phase: in.Phase, Tool: "naabu"})
		})
	}}
}
func platformError() error {
	return fmt.Errorf("SYN scan requires supported capture driver and elevated privileges on %s", runtime.GOOS)
}
