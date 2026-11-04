/*
MIT License

Copyright (c) 2026 Jan Van Herck (https://github.com/vhjan)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/*
Package updater implements the self-update logic: querying GitHub for the latest release, detecting how the tool
was installed, and running the matching package manager's upgrade command. It has no UI code; the callers decide
how to present results.
*/
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/vhjan/git-janitor/internal/buildinfo"
)

const (
	releasesAPI  = "https://api.github.com/repos/" + buildinfo.Repo + "/releases/latest"
	checkTimeout = 3 * time.Second
)

// PackageManager is a detected installation method and the command that upgrades the tool through it.
type PackageManager struct {
	Name    string
	Command []string
}

// Run executes the upgrade command, streaming its output to the given writers.
func (pm *PackageManager) Run(stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command(pm.Command[0], pm.Command[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// ManualCommand is the command to suggest when no package manager could be detected.
func ManualCommand() string {
	return "go install github.com/" + buildinfo.Repo + "@latest"
}

// Normalize strips a leading "v" so "v1.2.3" and "1.2.3" compare equal.
func Normalize(v string) string {
	return strings.TrimPrefix(v, "v")
}

// Detect figures out how the tool was installed. It returns nil if no usable package manager is found.
func Detect() *PackageManager {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	exeLower := strings.ToLower(exe)

	switch runtime.GOOS {
	case "darwin", "linux":
		// Homebrew: the executable lives under the Homebrew prefix
		if brewPath, err := exec.LookPath("brew"); err == nil {
			brewPrefixDir := strings.TrimSuffix(brewPath, "/bin/brew")
			upgrade := []string{"brew", "upgrade", "vhjan/tap/" + buildinfo.Name}
			if strings.HasPrefix(exeLower, strings.ToLower(brewPrefixDir)) {
				return &PackageManager{Name: "Homebrew", Command: upgrade}
			}
			// Also ask brew where it thinks the formula lives
			out, err := exec.Command("brew", "--prefix", buildinfo.Name).Output()
			if err == nil && strings.TrimSpace(string(out)) != "" {
				return &PackageManager{Name: "Homebrew", Command: upgrade}
			}
		}

	case "windows":
		// Scoop: the binary typically lives under %USERPROFILE%\scoop
		if strings.Contains(exeLower, "scoop") {
			return &PackageManager{Name: "Scoop", Command: []string{"scoop", "update", buildinfo.Name}}
		}
	}

	// Fallback: go install
	if _, err := exec.LookPath("go"); err == nil {
		return &PackageManager{Name: "go install", Command: strings.Fields(ManualCommand())}
	}

	return nil
}

// LatestVersion queries the GitHub releases API and returns the latest release tag without a leading "v".
func LatestVersion() (string, error) {
	req, err := http.NewRequest("GET", releasesAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", buildinfo.Name+"/"+buildinfo.Version)

	resp, err := (&http.Client{Timeout: checkTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return Normalize(release.TagName), nil
}

// CheckForUpdate returns the latest version when it differs from the running one. It returns "" for dev builds,
// when already up to date, or when the check fails (offline is not an error worth surfacing).
func CheckForUpdate() string {
	if buildinfo.IsDev() {
		return ""
	}
	latest, err := LatestVersion()
	if err != nil || latest == "" {
		return ""
	}
	if Normalize(latest) == Normalize(buildinfo.Version) {
		return ""
	}
	return latest
}
