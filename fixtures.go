// Package tollgate exposes repository data that the binary embeds.
package tollgate

import (
	"embed"
	"io/fs"
)

//go:embed testdata/fixtures/*.json
var fixtures embed.FS

// Fixtures returns the cleaned request fixtures of testdata/fixtures. The
// dry run of a configuration import runs them through the active and the
// candidate configuration (ADR-0008).
func Fixtures() fs.FS {
	sub, err := fs.Sub(fixtures, "testdata/fixtures")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}
