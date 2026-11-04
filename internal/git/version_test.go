package git

import (
	"errors"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{"git version 2.55.0.windows.4\n", Version{2, 55}, false},
		{"git version 2.39.3 (Apple Git-145)", Version{2, 39}, false},
		{"git version 2.23.0", Version{2, 23}, false},
		{"git version 1.8.3.1", Version{1, 8}, false},
		{"git version 2", Version{}, true},
		{"hg version 5.0", Version{}, true},
		{"", Version{}, true},
	}
	for _, tt := range tests {
		got, err := ParseVersion(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v (err=%v)", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b Version
		want bool
	}{
		{Version{2, 22}, MinVersion, true},
		{Version{2, 23}, MinVersion, false},
		{Version{2, 40}, MinVersion, false},
		{Version{1, 99}, MinVersion, true},
		{Version{3, 0}, MinVersion, false},
	}
	for _, c := range cases {
		if got := c.a.Less(c.b); got != c.want {
			t.Errorf("%v.Less(%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestTooOldErrorMessage(t *testing.T) {
	msg := (&TooOldError{Found: Version{2, 20}}).Error()
	for _, want := range []string{"2.20", "2.23", "too old"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q is missing %q", msg, want)
		}
	}
}

func TestInstalledGitIsSupported(t *testing.T) {
	r := newTestRepo(t)
	v, err := r.Version()
	if err != nil {
		t.Fatal(err)
	}
	if v.Less(MinVersion) {
		t.Fatalf("the git used to run these tests (%v) is older than the supported minimum %v", v, MinVersion)
	}
}

func TestOpenRejectsTooOldGit(t *testing.T) {
	r := newTestRepo(t) // also skips when git is missing

	old := MinVersion
	MinVersion = Version{Major: 99, Minor: 0} // pretend the installed git is too old
	t.Cleanup(func() { MinVersion = old })

	_, err := Open(r.Dir)
	var tooOld *TooOldError
	if !errors.As(err, &tooOld) {
		t.Fatalf("Open = %v, want *TooOldError", err)
	}
	if tooOld.Found.Less(old) {
		t.Errorf("reported version %v is older than the real minimum", tooOld.Found)
	}
}
