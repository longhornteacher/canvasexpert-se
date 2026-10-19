//go:build ignore

// Regenerates past_seeds.txt: the SHA-256 (CRLF normalized to LF) of every
// committed version of each Engine-managed rules file. Run from the repo root:
//
//	go run internal/playbooks/pastseeds_gen.go
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func git(args ...string) []byte {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		panic(fmt.Sprint(args, err))
	}
	return out
}

func main() {
	files := []string{"AGENTS.md", "CLAUDE.md", "TEACHER.md"}
	for _, p := range strings.Fields(string(git("log", "--format=", "--name-only", "--", "workspace-template/playbooks/"))) {
		files = append(files, strings.TrimPrefix(p, "workspace-template/"))
	}
	seen := map[string]bool{}
	var lines []string
	for _, rel := range files {
		for _, commit := range strings.Fields(string(git("log", "--format=%H", "--", "workspace-template/"+rel))) {
			blob, err := exec.Command("git", "cat-file", "blob", commit+":workspace-template/"+rel).Output()
			if err != nil {
				continue // deleted in that commit
			}
			line := fmt.Sprintf("%s %x", rel, sha256.Sum256(bytes.ReplaceAll(blob, []byte("\r\n"), []byte("\n"))))
			if !seen[line] {
				seen[line] = true
				lines = append(lines, line)
			}
		}
	}
	sort.Strings(lines)
	if err := os.WriteFile("internal/playbooks/past_seeds.txt", []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		panic(err)
	}
	fmt.Println("wrote", len(lines), "fingerprints")
}
