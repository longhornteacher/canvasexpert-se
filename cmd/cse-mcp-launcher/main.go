// cse-mcp-launcher contains no Engine. It forwards stdio to the authoritative
// executable in the selected UserInstall, so Engine upgrades need no extension update.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root := flag.String("install", "", "CE-SE UserInstall directory")
	flag.Parse()
	if *root == "" || !filepath.IsAbs(*root) || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "An absolute CE-SE UserInstall is required")
		os.Exit(2)
	}
	exe := filepath.Join(*root, "engine", "cse.exe")
	if _, err := os.Stat(filepath.Join(*root, "context", "courses.md")); err != nil {
		fmt.Fprintln(os.Stderr, "Selected UserInstall is missing context/courses.md")
		os.Exit(2)
	}
	cmd := exec.Command(exe, "--workspace", *root, "mcp")
	hide(cmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if e, ok := err.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(4)
	}
}
