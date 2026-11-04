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
Package buildinfo holds the application's identity and the build metadata injected at release time.

Version, Commit and Date are overwritten through ldflags (see .goreleaser.yaml and the Homebrew formula in
.github/workflows/release.yml), e.g.:

	-X github.com/vhjan/git-janitor/internal/buildinfo.Version=1.2.3
*/
package buildinfo

const (
	// Repo is the GitHub "owner/name" slug used for update checks.
	Repo = "vhjan/git-janitor"

	// Name is the canonical binary name. Because it starts with "git-", Git also exposes it as `git janitor`.
	Name = "git-janitor"

	// ShortName is the quick alias binary that is shipped alongside the canonical one.
	ShortName = "gj"
)

// Build metadata. Overwritten via ldflags during release builds.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// IsDev reports whether this is a local, non-release build.
func IsDev() bool {
	return Version == "" || Version == "dev"
}
