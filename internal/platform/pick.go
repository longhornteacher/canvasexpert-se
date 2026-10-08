package platform

import (
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
	"unicode/utf16"
)

const pickFolderScript = `
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form
$owner.TopMost = $true
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = 'Choose your synced CanvasExpert-SE workspace folder. Private data lives there, so pick a folder that is not the program folder.'
$dialog.ShowNewFolderButton = $true
if ($dialog.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $dialog.SelectedPath }
`

// PickFolder shows the native Windows folder chooser and returns the chosen
// path, or "" when the dialog is cancelled.
func PickFolder(ctx context.Context) (string, error) {
	units := utf16.Encode([]rune(pickFolderScript))
	raw := make([]byte, 0, len(units)*2)
	for _, u := range units {
		raw = append(raw, byte(u), byte(u>>8))
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-EncodedCommand", base64.StdEncoding.EncodeToString(raw))
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("the folder chooser could not open; paste the folder path instead")
	}
	return strings.TrimSpace(strings.TrimPrefix(string(out), string(rune(0xFEFF)))), nil
}
