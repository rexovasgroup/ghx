// Command idtype-checker runs the idtype analyzer to flag struct fields
// representing GitHub database IDs that use int instead of int64.
//
// Usage:
//
//	go run ./cmd/idtype-checker ./...
package main

import (
	"github.com/cli/cli/v2/pkg/linter/idtype"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(idtype.Analyzer)
}
