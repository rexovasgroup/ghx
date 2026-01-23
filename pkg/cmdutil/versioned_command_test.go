package cmdutil

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func TestGHESVersionConstraint(t *testing.T) {
	tests := []struct {
		name       string
		operator   string
		constraint string
		testVer    string
		want       bool
	}{
		{
			name:       "less than - true",
			operator:   "<",
			constraint: "3.17.0",
			testVer:    "3.16.0",
			want:       true,
		},
		{
			name:       "less than - false",
			operator:   "<",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       false,
		},
		{
			name:       "less than - false (greater)",
			operator:   "<",
			constraint: "3.17.0",
			testVer:    "3.18.0",
			want:       false,
		},
		{
			name:       "less than or equal - true (less)",
			operator:   "<=",
			constraint: "3.17.0",
			testVer:    "3.16.0",
			want:       true,
		},
		{
			name:       "less than or equal - true (equal)",
			operator:   "<=",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       true,
		},
		{
			name:       "less than or equal - false",
			operator:   "<=",
			constraint: "3.17.0",
			testVer:    "3.18.0",
			want:       false,
		},
		{
			name:       "greater than - true",
			operator:   ">",
			constraint: "3.16.0",
			testVer:    "3.17.0",
			want:       true,
		},
		{
			name:       "greater than - false",
			operator:   ">",
			constraint: "3.17.0",
			testVer:    "3.16.0",
			want:       false,
		},
		{
			name:       "greater than or equal - true (greater)",
			operator:   ">=",
			constraint: "3.16.0",
			testVer:    "3.17.0",
			want:       true,
		},
		{
			name:       "greater than or equal - true (equal)",
			operator:   ">=",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       true,
		},
		{
			name:       "equal - true",
			operator:   "==",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       true,
		},
		{
			name:       "equal - false",
			operator:   "==",
			constraint: "3.17.0",
			testVer:    "3.16.0",
			want:       false,
		},
		{
			name:       "not equal - true",
			operator:   "!=",
			constraint: "3.17.0",
			testVer:    "3.16.0",
			want:       true,
		},
		{
			name:       "not equal - false",
			operator:   "!=",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       false,
		},
		{
			name:       "invalid operator",
			operator:   "~",
			constraint: "3.17.0",
			testVer:    "3.17.0",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := GHESVersionConstraint(tt.operator, tt.constraint)
			v, err := version.NewVersion(tt.testVer)
			if err != nil {
				t.Fatalf("failed to parse test version: %v", err)
			}
			got := constraint(v)
			if got != tt.want {
				t.Errorf("GHESVersionConstraint(%q, %q)(%q) = %v, want %v",
					tt.operator, tt.constraint, tt.testVer, got, tt.want)
			}
		})
	}
}

func TestGHESVersionConstraint_InvalidConstraint(t *testing.T) {
	// Invalid constraint version should return a constraint that never matches
	constraint := GHESVersionConstraint("<", "not-a-version")
	v, _ := version.NewVersion("3.17.0")
	if constraint(v) {
		t.Error("expected constraint with invalid version to return false")
	}
}

func TestGHESVersionRange(t *testing.T) {
	tests := []struct {
		name    string
		min     string
		max     string
		testVer string
		want    bool
	}{
		{
			name:    "in range",
			min:     "3.16.0",
			max:     "3.18.0",
			testVer: "3.17.0",
			want:    true,
		},
		{
			name:    "at min boundary (inclusive)",
			min:     "3.16.0",
			max:     "3.18.0",
			testVer: "3.16.0",
			want:    true,
		},
		{
			name:    "at max boundary (exclusive)",
			min:     "3.16.0",
			max:     "3.18.0",
			testVer: "3.18.0",
			want:    false,
		},
		{
			name:    "below range",
			min:     "3.16.0",
			max:     "3.18.0",
			testVer: "3.15.0",
			want:    false,
		},
		{
			name:    "above range",
			min:     "3.16.0",
			max:     "3.18.0",
			testVer: "3.19.0",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := GHESVersionRange(tt.min, tt.max)
			v, err := version.NewVersion(tt.testVer)
			if err != nil {
				t.Fatalf("failed to parse test version: %v", err)
			}
			got := constraint(v)
			if got != tt.want {
				t.Errorf("GHESVersionRange(%q, %q)(%q) = %v, want %v",
					tt.min, tt.max, tt.testVer, got, tt.want)
			}
		})
	}
}

func TestGHESVersionRange_InvalidVersions(t *testing.T) {
	tests := []struct {
		name string
		min  string
		max  string
	}{
		{"invalid min", "not-a-version", "3.18.0"},
		{"invalid max", "3.16.0", "not-a-version"},
		{"both invalid", "not-a-version", "also-not-a-version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint := GHESVersionRange(tt.min, tt.max)
			v, _ := version.NewVersion("3.17.0")
			if constraint(v) {
				t.Error("expected constraint with invalid version to return false")
			}
		})
	}
}
