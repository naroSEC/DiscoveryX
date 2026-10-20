package main

import (
	"context"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/cli"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "discoveryx:", err)
		os.Exit(1)
	}
}
