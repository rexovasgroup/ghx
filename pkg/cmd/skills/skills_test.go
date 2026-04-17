package skills

import (
	"testing"

	"github.com/cli/cli/v2/internal/gh/ghtelemetry"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmd/skills/install"
	"github.com/cli/cli/v2/pkg/cmd/skills/preview"
	"github.com/cli/cli/v2/pkg/cmd/skills/search"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCmdSkills_DoesNotSetSampleRateOnConstruction is a regression
// guard. NewCmdSkills is called unconditionally at root command
// construction time from NewCmdRoot, before cobra has matched any
// arguments. If SetSampleRate were called during construction, every
// gh invocation (e.g. `gh issue list`, `gh pr view`, tab completion)
// would silently bump the telemetry sample rate to SAMPLE_ALL, breaking
// the global sampling strategy for commands unrelated to skills.
//
// The sample rate must only be raised when a skill subcommand actually
// runs. If this test fails, something in the skill command tree is
// invoking SetSampleRate at construction time again.
func TestNewCmdSkills_DoesNotSetSampleRateOnConstruction(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
		Prompter:  &prompter.PrompterMock{},
	}
	spy := &telemetry.CommandRecorderSpy{}

	_ = NewCmdSkills(f, spy)

	require.Zero(t, spy.SampleRateCalls,
		"constructing the skill command must not set the sample rate; "+
			"it would leak SAMPLE_ALL into every gh invocation because "+
			"NewCmdSkills is called unconditionally from NewCmdRoot")
}

// TestSkillSubcommands_SetSampleRateOnRun verifies the intended
// behavior: when a user actually runs a skill subcommand, the sample
// rate is bumped to SAMPLE_ALL so the preview feature can be fully
// observed. Each subcommand that records telemetry is checked so a
// new subcommand can't silently regress this.
//
// We invoke each subcommand's constructor with a runF hook that
// short-circuits the real implementation. Cobra still invokes the
// RunE body, which is where SetSampleRate lives, before delegating
// to runF.
func TestSkillSubcommands_SetSampleRateOnRun(t *testing.T) {
	tests := []struct {
		name string
		exec func(t *testing.T, f *cmdutil.Factory, telem ghtelemetry.CommandRecorder)
	}{
		{
			name: "install",
			exec: func(t *testing.T, f *cmdutil.Factory, telem ghtelemetry.CommandRecorder) {
				var called bool
				cmd := install.NewCmdInstall(f, telem, func(*install.InstallOptions) error {
					called = true
					return nil
				})
				cmd.SetArgs([]string{"monalisa/repo", "my-skill"})
				require.NoError(t, cmd.Execute())
				require.True(t, called, "runF should have been invoked")
			},
		},
		{
			name: "preview",
			exec: func(t *testing.T, f *cmdutil.Factory, telem ghtelemetry.CommandRecorder) {
				var called bool
				cmd := preview.NewCmdPreview(f, telem, func(*preview.PreviewOptions) error {
					called = true
					return nil
				})
				cmd.SetArgs([]string{"monalisa/repo", "my-skill"})
				require.NoError(t, cmd.Execute())
				require.True(t, called, "runF should have been invoked")
			},
		},
		{
			name: "search",
			exec: func(t *testing.T, f *cmdutil.Factory, telem ghtelemetry.CommandRecorder) {
				var called bool
				cmd := search.NewCmdSearch(f, telem, func(*search.SearchOptions) error {
					called = true
					return nil
				})
				cmd.SetArgs([]string{"terraform"})
				require.NoError(t, cmd.Execute())
				require.True(t, called, "runF should have been invoked")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: ios,
				Prompter:  &prompter.PrompterMock{},
			}
			spy := &telemetry.CommandRecorderSpy{}

			tt.exec(t, f, spy)

			assert.GreaterOrEqual(t, spy.SampleRateCalls, 1,
				"%s subcommand must call SetSampleRate at the start of RunE", tt.name)
			assert.Equal(t, ghtelemetry.SAMPLE_ALL, spy.LastSampleRate,
				"%s subcommand must raise the sample rate to SAMPLE_ALL", tt.name)
		})
	}
}
