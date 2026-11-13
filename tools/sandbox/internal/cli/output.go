package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

func clock(at *time.Time) string {
	if at == nil {
		return "-"
	}
	return at.Local().Format("15:04:05")
}

func keyPlace(s sandbox.Sandbox) string {
	if s.HasKey {
		return "this machine"
	}
	return "elsewhere"
}

func (c CLI) printTable(sandboxes []sandbox.Sandbox) {
	if len(sandboxes) == 0 {
		fmt.Fprintln(c.Stdout, "No sandboxes.")
		return
	}

	table := tabwriter.NewWriter(c.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSTATE\tSTARTED\tSTOPS ITSELF\tSSH KEY")
	for _, s := range sandboxes {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.State, s.CreatedAt.Local().Format("Jan 2 15:04"), clock(s.AutoStopAt), keyPlace(s))
	}
	table.Flush()
}

func (c CLI) printDetails(s sandbox.Sandbox) {
	table := tabwriter.NewWriter(c.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(table, "ID\t%s\n", s.ID)
	fmt.Fprintf(table, "State\t%s\n", s.State)
	fmt.Fprintf(table, "Backend\t%s\n", s.Backend)
	fmt.Fprintf(table, "Started\t%s\n", s.CreatedAt.Local().Format("Jan 2 15:04:05"))
	fmt.Fprintf(table, "Stops itself\t%s\n", clock(s.AutoStopAt))
	fmt.Fprintf(table, "Hard limit\t%s\n", clock(s.HardStopAt))
	fmt.Fprintf(table, "SSH key\t%s\n", keyPlace(s))
	fmt.Fprintf(table, "Run\t%s\n", s.URL)
	table.Flush()
}
