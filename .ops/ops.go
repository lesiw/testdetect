// op is the command line for project operations.
package main

import (
	"os"

	"labs.lesiw.io/ops/goapp"
	"lesiw.io/ops"
)

// Ops is the set of operations for this project.
type Ops struct{ goapp.Ops }

func main() {
	goapp.Name = "testdetect"
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "build")
	}
	ops.Handle(Ops{})
}
