package main

import (
	"flag"
	"fmt"
	"io"

	"canvasexpertse/internal/workflow"
)

func parseAssignmentCopyOptions(args []string) (workflow.AssignmentCopyOptions, error) {
	var opts workflow.AssignmentCopyOptions
	f := flag.NewFlagSet("copy-assignment", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.Int64Var(&opts.ModuleID, "module-id", 0, "existing destination module")
	f.Int64Var(&opts.AssignmentGroupID, "assignment-group-id", 0, "destination assignment group")
	f.BoolVar(&opts.Published, "published", false, "publish after every requested piece succeeds")
	if err := f.Parse(args); err != nil {
		return opts, err
	}
	if f.NArg() != 0 || opts.ModuleID < 0 || opts.AssignmentGroupID < 0 {
		return opts, fmt.Errorf("copy-assignment SOURCE QUERY TARGET [--module-id ID] [--assignment-group-id ID] [--published]; IDs must be positive")
	}
	return opts, nil
}
