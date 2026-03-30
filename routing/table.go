package routing

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// TableEntry represents a single row in the route table.
type TableEntry struct {
	Method     string
	Path       string
	Handler    string
	Format     string
	Middleware []string
	Template   string
}

// RouteTable holds all registered route entries for display.
type RouteTable struct {
	Entries []TableEntry
}

// Add appends an entry to the route table.
func (t *RouteTable) Add(entry TableEntry) {
	t.Entries = append(t.Entries, entry)
}

// Print writes a formatted route table to the writer.
func (t *RouteTable) Print(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "METHOD\tPATH\tHANDLER\tFORMAT\tMIDDLEWARE\tTEMPLATE")
	fmt.Fprintln(tw, "------\t----\t-------\t------\t----------\t--------")

	for _, e := range t.Entries {
		mw := ""
		if len(e.Middleware) > 0 {
			mw = strings.Join(e.Middleware, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Method, e.Path, e.Handler, e.Format, mw, e.Template)
	}
	tw.Flush()
}

// String returns the route table as a formatted string.
func (t *RouteTable) String() string {
	var buf strings.Builder
	t.Print(&buf)
	return buf.String()
}
