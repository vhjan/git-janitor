package branches

import (
	"testing"
	"time"
)

func TestIsProtected(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		current  string
		def      string
		patterns []string
		want     bool
	}{
		{"protect dev", "dev", "feature", "main", nil, true},
		{"protect current branch", "feature", "feature", "main", nil, true},
		{"protect default branch", "main", "feature", "main", nil, true},
		{"protect exact custom pattern", "qa", "feature", "main", []string{"qa"}, true},
		{"protect wildcard custom pattern", "release-1.0", "feature", "main", []string{"release-*"}, true},
		{"do not protect other branches", "fix-bug", "feature", "main", []string{"release-*"}, false},
		{"multiple custom patterns", "staging", "feature", "main", []string{"qa", "staging", "release-*"}, true},
		{"invalid pattern protects nothing", "anything", "feature", "main", []string{"["}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsProtected(tt.branch, tt.current, tt.def, tt.patterns); got != tt.want {
				t.Errorf("IsProtected(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	now := time.Unix(1_000_000_000, 0)
	day := int64(24 * 60 * 60)

	tests := []struct {
		name   string
		commit int64
		days   float64
		want   bool
	}{
		{"recent commit", now.Unix() - 5*day, 30, false},
		{"old commit", now.Unix() - 31*day, 30, true},
		{"unknown timestamp is never stale", 0, 30, false},
		{"fractional threshold", now.Unix() - day, 0.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStale(tt.commit, now, tt.days); got != tt.want {
				t.Errorf("IsStale = %v, want %v", got, tt.want)
			}
		})
	}
}
