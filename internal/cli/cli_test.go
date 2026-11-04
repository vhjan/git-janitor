package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	code = Execute("git-janitor", args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestHelpListsEverything(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		code, out, _ := run(args...)
		if code != 0 {
			t.Errorf("%v exited %d, want 0", args, code)
		}
		for _, want := range []string{"COMMANDS:", "FLAGS:", "INTERACTIVE KEYBINDINGS:", "update", "--dry-run", "--stale-days", "-h, --help", "gj"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v output is missing %q", args, want)
			}
		}
	}
}

func TestVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"-v"}, {"--version"}} {
		code, out, _ := run(args...)
		if code != 0 || !strings.Contains(out, "version") {
			t.Errorf("%v: code=%d out=%q", args, code, out)
		}
	}
}

func TestUnknownCommandAndFlag(t *testing.T) {
	if code, _, errOut := run("frobnicate"); code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("unknown command: code=%d stderr=%q", code, errOut)
	}
	if code, _, errOut := run("--nope"); code != 2 || errOut == "" {
		t.Errorf("unknown flag: code=%d stderr=%q", code, errOut)
	}
}

func TestFlagsBuildConfig(t *testing.T) {
	f := newRootFlags()
	if err := f.fs.Parse([]string{"--dry-run", "--protect", "qa, release-*", "--stale-days", "14"}); err != nil {
		t.Fatal(err)
	}
	cfg := f.config()
	if !cfg.DryRun || cfg.StaleDays != 14 || len(cfg.Protect) != 2 || cfg.Protect[1] != "release-*" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestProgramName(t *testing.T) {
	cases := map[string]string{
		"git-janitor":                "git-janitor",
		"/usr/local/bin/git-janitor": "git-janitor",
		"gj":                         "gj",
		"GJ.EXE":                     "gj",
		"/tmp/go-build123/exe/main":  "git-janitor",
	}
	for in, want := range cases {
		if got := programName(in); got != want {
			t.Errorf("programName(%q) = %q, want %q", in, got, want)
		}
	}
}
