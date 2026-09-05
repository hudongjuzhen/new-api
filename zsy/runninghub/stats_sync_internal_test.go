package runninghub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Internal tests for the pure helpers behind stats aggregation and the
// channel sync. They encode the channel models-list convention
// (comma-separated, as model.Channel.GetModelList parses it) and the
// dev-plan model-name prefix fallback.

func TestSplitChannelModels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: []string{}},
		{name: "single", input: "1877265245566922753", want: []string{"1877265245566922753"}},
		{name: "comma separated", input: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "trailing comma trimmed", input: "a,b,", want: []string{"a", "b"}},
		{name: "whitespace stripped", input: " a , b ", want: []string{"a", "b"}},
		{name: "duplicates dropped", input: "a,b,a,b", want: []string{"a", "b"}},
		{name: "empty segments dropped", input: "a,,b", want: []string{"a", "b"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := splitChannelModels(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestModelToUpstreamID(t *testing.T) {
	t.Parallel()
	// bare UpstreamID passes through untouched (current plugin convention)
	assert.Equal(t, "1877265245566922753", modelToUpstreamID("1877265245566922753"))
	// dev-plan prefixed variants resolve to the upstream id
	assert.Equal(t, "12345", modelToUpstreamID("rh-aiapp-12345"))
	assert.Equal(t, "67890", modelToUpstreamID("rh-workflow-67890"))
	assert.Equal(t, "24680", modelToUpstreamID("rh-model-24680"))
	// non-plugin models are returned as-is so they surface as orphans
	assert.Equal(t, "gpt-4o", modelToUpstreamID("gpt-4o"))
}

func TestSyncActionConstants(t *testing.T) {
	t.Parallel()
	// lock the wire values: the admin UI switches on them
	assert.Equal(t, "synced_to_channel", syncActionSyncedChannel)
	assert.Equal(t, "ok", syncActionOK)
	assert.Equal(t, "orphan_model", syncActionOrphanModel)
}
