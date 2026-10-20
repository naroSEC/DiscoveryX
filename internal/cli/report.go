package cli

import (
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/reporting"
	"github.com/spf13/cobra"
	"io"
)

func reportCommand(out io.Writer) *cobra.Command {
	var run string
	cmd := &cobra.Command{Use: "report", Short: "Generate an offline HTML report from an existing run without scanning", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		path, err := reporting.Regenerate(run)
		if err == nil {
			fmt.Fprintf(out, "HTML report: %s\n", path)
		}
		return err
	}}
	cmd.Flags().StringVar(&run, "run", "", "Existing run directory")
	_ = cmd.MarkFlagRequired("run")
	return cmd
}
