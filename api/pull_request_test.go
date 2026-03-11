package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChecksStatus_NoCheckRunsOrStatusContexts(t *testing.T) {
	t.Parallel()

	payload := `
{ "statusCheckRollup": { "nodes": [] } }
`
	var pr PullRequest
	require.NoError(t, json.Unmarshal([]byte(payload), &pr))

	expectedChecksStatus := PullRequestChecksStatus{
		Pending: 0,
		Failing: 0,
		Passing: 0,
		Total:   0,
	}
	require.Equal(t, expectedChecksStatus, pr.ChecksStatus())
}

func TestChecksStatus_SummarisingCheckRunAndStatusContextCountsByState(t *testing.T) {
	t.Parallel()

	payload := `
{ "statusCheckRollup": { "nodes": [{ "commit": {
"statusCheckRollup": {
"contexts": {
"checkRunCount": 14,
"checkRunCountsByState": [
{
"state": "ACTION_REQUIRED",
"count": 1
},
{
"state": "CANCELLED",
"count": 1
},
{
"state": "COMPLETED",
"count": 1
},
{
"state": "FAILURE",
"count": 1
},
{
"state": "IN_PROGRESS",
"count": 1
},
{
"state": "NEUTRAL",
"count": 1
},
{
"state": "PENDING",
"count": 1
},
{
"state": "QUEUED",
"count": 1
},
{
"state": "SKIPPED",
"count": 1
},
{
"state": "STALE",
"count": 1
},
{
"state": "STARTUP_FAILURE",
"count": 1
},
{
"state": "SUCCESS",
"count": 1
},
{
"state": "TIMED_OUT",
"count": 1
},
{
"state": "WAITING",
"count": 1
},
{
"state": "AnUnrecognizedStateJustForThisTest",
"count": 1
}
],
"statusContextCount": 6,
"statusContextCountsByState": [
{
"state": "EXPECTED",
"count": 1
},
{
"state": "ERROR",
"count": 1
},
{
"state": "FAILURE",
"count": 1
},
{
"state": "PENDING",
"count": 1
},
{
"state": "SUCCESS",
"count": 1
},
{
"state": "AnUnrecognizedStateJustForThisTest",
"count": 1
}
]
}
}
} }] } }
`

	var pr PullRequest
	require.NoError(t, json.Unmarshal([]byte(payload), &pr))

	expectedChecksStatus := PullRequestChecksStatus{
		Pending: 11,
		Failing: 5,
		Passing: 4,
		Total:   19,
	}
	require.Equal(t, expectedChecksStatus, pr.ChecksStatus())
}

func TestEliminateDuplicateChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		checkContexts []CheckContext
		want          []CheckContext
	}{
		{
			name: "duplicate CheckRun keeps most recent",
			checkContexts: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "lint",
					Status:     "COMPLETED",
					Conclusion: "FAILURE",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
				{
					TypeName:   "CheckRun",
					Name:       "lint",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 2, 2, 2, 2, 2, 2, time.UTC),
				},
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
			},
			want: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "lint",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 2, 2, 2, 2, 2, 2, time.UTC),
				},
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
			},
		},
		{
			name: "duplicate StatusContext keeps most recent",
			checkContexts: []CheckContext{
				{
					TypeName:  "StatusContext",
					Context:   "ci/test",
					State:     "FAILURE",
					StartedAt: time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
				{
					TypeName:  "StatusContext",
					Context:   "ci/test",
					State:     "SUCCESS",
					StartedAt: time.Date(2022, 2, 2, 2, 2, 2, 2, time.UTC),
				},
			},
			want: []CheckContext{
				{
					TypeName:  "StatusContext",
					Context:   "ci/test",
					State:     "SUCCESS",
					StartedAt: time.Date(2022, 2, 2, 2, 2, 2, 2, time.UTC),
				},
			},
		},
		{
			name: "unique checks are preserved",
			checkContexts: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
				{
					TypeName:  "StatusContext",
					Context:   "ci/test",
					State:     "SUCCESS",
					StartedAt: time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
			},
			want: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
				{
					TypeName:  "StatusContext",
					Context:   "ci/test",
					State:     "SUCCESS",
					StartedAt: time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
				},
			},
		},
		{
			name: "different workflow names are not deduplicated",
			checkContexts: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
					CheckSuite: CheckSuite{WorkflowRun: WorkflowRun{Event: "push", Workflow: Workflow{Name: "CI"}}},
				},
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
					CheckSuite: CheckSuite{WorkflowRun: WorkflowRun{Event: "push", Workflow: Workflow{Name: "Release"}}},
				},
			},
			want: []CheckContext{
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
					CheckSuite: CheckSuite{WorkflowRun: WorkflowRun{Event: "push", Workflow: Workflow{Name: "CI"}}},
				},
				{
					TypeName:   "CheckRun",
					Name:       "build",
					Status:     "COMPLETED",
					Conclusion: "SUCCESS",
					StartedAt:  time.Date(2022, 1, 1, 1, 1, 1, 1, time.UTC),
					CheckSuite: CheckSuite{WorkflowRun: WorkflowRun{Event: "push", Workflow: Workflow{Name: "Release"}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EliminateDuplicateChecks(tt.checkContexts)
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("got EliminateDuplicateChecks %+v, want %+v", got, tt.want)
			}
		})
	}
}
