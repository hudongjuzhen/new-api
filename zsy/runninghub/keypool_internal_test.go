package runninghub

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Internal tests for the pure helpers behind the app-level keypool refresh.
// These encode contracts the admin UI and the keypool runtime depend on; they
// never touch the database.

func TestSplitChannelKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty string", input: "", want: []string{}},
		{name: "single key", input: "abc123", want: []string{"abc123"}},
		{name: "newline separated", input: "key1\nkey2\nkey3", want: []string{"key1", "key2", "key3"}},
		{name: "trailing newline trimmed", input: "key1\nkey2\n", want: []string{"key1", "key2"}},
		{name: "surrounding whitespace stripped", input: "  key1 \n\tkey2\n", want: []string{"key1", "key2"}},
		{name: "blank lines dropped", input: "key1\n\n\nkey2", want: []string{"key1", "key2"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := splitChannelKeys(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMaskKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},
		{input: "short", want: "****"},
		{input: "12345678", want: "****"},
		{input: "abcdefgh", want: "****"},
		{input: "sk-1234567890abcdef", want: "sk-1****cdef"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, maskKey(tc.input), "maskKey(%q)", tc.input)
	}
	// masked output never contains the middle of the key
	long := "rh-0123456789abcdefghijklmnopqrstuvwxyz"
	masked := maskKey(long)
	assert.False(t, strings.Contains(masked, "0123456789"))
}

func TestIsTerminalTaskStatus(t *testing.T) {
	t.Parallel()
	assert.True(t, isTerminalTaskStatus(model.TaskStatusSuccess))
	assert.True(t, isTerminalTaskStatus(model.TaskStatusFailure))
	// everything still in flight keeps the keypool slot occupied
	for _, s := range []model.TaskStatus{
		model.TaskStatusNotStart,
		model.TaskStatusSubmitted,
		model.TaskStatusQueued,
		model.TaskStatusInProgress,
		model.TaskStatusUnknown,
	} {
		assert.False(t, isTerminalTaskStatus(s), "status %s must not be terminal", s)
	}
}
