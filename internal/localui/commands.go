package localui

import (
	"fmt"
	"os"
	"strings"

	"canvasexpertse/internal/workspace"
)

// commandsFile lists the Home page commands: prompts a teacher copies into
// their teaching app. It is teacher context, so it follows the rules authority
// and the agent edits it on request.
const commandsFile = "context/commands.md"

type homeCommand struct {
	Name   string `yaml:"name"`
	About  string `yaml:"about"`
	Prompt string `yaml:"prompt"`
}

type commandsDoc struct {
	Schema   int           `yaml:"schema"`
	Commands []homeCommand `yaml:"commands"`
}

func (d commandsDoc) Validate() error {
	for i, c := range d.Commands {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Prompt) == "" {
			return fmt.Errorf("command %d needs a name and a prompt", i+1)
		}
	}
	return nil
}

// homeCommands reads the teacher's commands. A missing or unreadable file
// falls back to this version's seed, with a note naming the problem so the
// teacher can ask their teaching app to fix it.
func homeCommands(root string) ([]homeCommand, string) {
	var doc commandsDoc
	var note string
	if root != "" {
		_, err := workspace.ReadTeacherDocument(root, commandsFile, &doc)
		if err == nil {
			return doc.Commands, ""
		}
		if !os.IsNotExist(err) {
			note = "Your Home commands could not be read, so the starter set is showing. Ask your teaching app to fix " + commandsFile + ": " + err.Error()
		}
	}
	b, err := seedBytes(commandsFile)
	if err == nil {
		_, err = workspace.ParseDocument(b, &doc)
	}
	if err != nil {
		return nil, note
	}
	return doc.Commands, note
}

// fillPrompt puts a course into a command prompt; the Home script does the
// same when the teacher picks another course or command.
func fillPrompt(prompt, course string) string {
	if course == "" {
		course = "one of my courses"
	}
	return strings.ReplaceAll(prompt, "{course}", course)
}
