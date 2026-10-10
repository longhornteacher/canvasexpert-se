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
Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class Fg { [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h); [DllImport("user32.dll")] public static extern void keybd_event(byte k, byte s, uint f, UIntPtr e); }'
# A visible, topmost, invisible-looking owner keeps the dialog above the browser even
# though this process was started in the background (Windows blocks background focus steals).
$owner = New-Object System.Windows.Forms.Form
$owner.Text = 'CanvasExpert-SE'
$owner.TopMost = $true
$owner.ShowInTaskbar = $false
$owner.FormBorderStyle = 'None'
$owner.StartPosition = 'CenterScreen'
$owner.Size = New-Object System.Drawing.Size(1, 1)
$owner.Opacity = 0
$owner.Show()
[Fg]::keybd_event(0x12, 0, 0, [UIntPtr]::Zero)
[Fg]::keybd_event(0x12, 0, 2, [UIntPtr]::Zero)
[void][Fg]::SetForegroundWindow($owner.Handle)
$owner.Activate()
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = 'Choose your CanvasExpert-SE workspace folder.'
$dialog.ShowNewFolderButton = $true
if ($dialog.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $dialog.SelectedPath }
$owner.Dispose()
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
