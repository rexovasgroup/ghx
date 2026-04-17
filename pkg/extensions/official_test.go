package extensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOfficialExtension_Repository(t *testing.T) {
	ext := &OfficialExtension{Name: "stack", Owner: "github", Repo: "gh-stack"}
	repo := ext.Repository()
	assert.Equal(t, "github", repo.RepoOwner())
	assert.Equal(t, "gh-stack", repo.RepoName())
	assert.Equal(t, "github.com", repo.RepoHost())
}

func TestIsOfficial(t *testing.T) {
	tests := []struct {
		name    string
		extName string
		want    bool
	}{
		{
			name:    "known official extension matches",
			extName: "stack",
			want:    true,
		},
		{
			name:    "mixed-case name still matches",
			extName: "STACK",
			want:    true,
		},
		{
			name:    "unknown name is not official",
			extName: "not-a-real-extension",
			want:    false,
		},
		{
			name:    "empty name is not official",
			extName: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsOfficial(tt.extName))
		})
	}
}
