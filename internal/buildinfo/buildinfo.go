package buildinfo

import (
	"fmt"
	"io"
)

// Info describes build-time metadata.
// Values are expected to be injected via -ldflags into main package variables.
type Info struct {
	Version string
	Date    string
	Commit  string
}

func normalize(v string) string {
	if v == "" {
		return "N/A"
	}
	return v
}

// Print writes normalized build info to w in a stable format.
func Print(w io.Writer, info Info) {
	fmt.Fprintln(w, "Build version:", normalize(info.Version))
	fmt.Fprintln(w, "Build date:", normalize(info.Date))
	fmt.Fprintln(w, "Build commit:", normalize(info.Commit))
}

