package engine

import "github.com/naroSEC/DiscoveryX/internal/config"

type Stage struct {
	Name  string
	Tool  string
	Phase string
}

func Plan(c config.Config) []Stage {
	s := []Stage{}
	if c.WebOnly {
		s = append(s, Stage{Name: "httpx", Tool: "httpx"})
		if c.GHRecon == "on" {
			s = append(s, Stage{Name: "ghrecon", Tool: "ghrecon"})
		}
		if c.Crawl == "on" {
			s = append(s, Stage{Name: "katana", Tool: "katana"})
		}
		if c.Directory == "on" {
			s = append(s, Stage{Name: "feroxbuster", Tool: "feroxbuster"})
		}
		if c.Nuclei == "on" {
			s = append(s, Stage{Name: "nuclei", Tool: "nuclei"})
		}
		return s
	}
	if c.PortScan == "first" || c.PortScan == "both" {
		s = append(s, Stage{"naabu_initial", "naabu", "initial"})
	}
	if c.Subfinder == "on" {
		s = append(s, Stage{Name: "subfinder", Tool: "subfinder"})
	}
	s = append(s, Stage{Name: "dnsx", Tool: "dnsx"}, Stage{Name: "httpx", Tool: "httpx"})
	if c.GHRecon == "on" {
		s = append(s, Stage{Name: "ghrecon", Tool: "ghrecon"})
	}
	if c.Crawl == "on" {
		s = append(s, Stage{Name: "katana", Tool: "katana"})
	}
	if c.Directory == "on" {
		s = append(s, Stage{Name: "feroxbuster", Tool: "feroxbuster"})
	}
	if c.Nuclei == "on" {
		s = append(s, Stage{Name: "nuclei", Tool: "nuclei"})
	}
	if c.PortScan == "last" || c.PortScan == "both" {
		s = append(s, Stage{"naabu_final", "naabu", "final"})
	}
	return s
}
