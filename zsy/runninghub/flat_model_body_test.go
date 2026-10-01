package runninghub

import (
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the two behaviors that make flat model-API apps (kind=model,
// schema imported from a flat JSON curl where every param has an empty NodeID)
// work end to end: the typed flat request-body assembly and the field-name
// billing references (@duration / len(@prompt)).

// flatModelSchema mirrors the shape the curl importer produces for a flat
// Model-API body: empty NodeIDs, FieldName == the upstream JSON key.
func flatModelSchema() []rhparser.SchemaParam {
	max30 := 30.0
	return []rhparser.SchemaParam{
		{FieldName: "fileUrl", Label: "文件", Type: "video", Required: true},
		{FieldName: "model", Label: "模型", Type: "select", Required: true,
			Options: []rhparser.SchemaParamOption{{Label: "max", Value: "max"}}},
		{FieldName: "resolution", Label: "分辨率", Type: "select", Default: "1080p",
			Options: []rhparser.SchemaParamOption{{Label: "1080p", Value: "1080p"}}},
		{FieldName: "duration", Label: "时长", Type: "number", Required: true, Max: &max30},
		{FieldName: "enableHDR", Label: "HDR", Type: "switch"},
	}
}

func TestBuildFlatModelBody_TypedFlatBody(t *testing.T) {
	values := map[string]any{
		".fileUrl":   "https://example.com/in.mp4",
		".model":     "max",
		".duration":  float64(10),
		".enableHDR": true,
		// Unknown keys and empty FieldName params must be dropped silently,
		// matching the nodeInfoList path semantics.
		".unknownKey": "x",
	}
	body, err := buildFlatModelBody(flatModelSchema(), values)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"fileUrl":   "https://example.com/in.mp4",
		"model":     "max",
		"duration":  float64(10),
		"enableHDR": true,
	}, body)
}

func TestBuildFlatModelBody_RejectsBoundAndOptionViolations(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]any
		errHas string
	}{
		{
			name:   "number above schema max",
			values: map[string]any{".duration": "60"},
			errHas: "不能大于",
		},
		{
			name:   "select value not in options",
			values: map[string]any{".model": "bogus"},
			errHas: "非法选项值",
		},
		{
			name:   "switch value not boolean",
			values: map[string]any{".enableHDR": "yes"},
			errHas: "必须为布尔值",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildFlatModelBody(flatModelSchema(), tc.values)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errHas)
		})
	}
}

func TestSecondsFromExpr_FlatFieldNameReference(t *testing.T) {
	schema := []rhparser.SchemaParam{
		{FieldName: "duration", Label: "时长", Type: "number"},
	}
	values := map[string]any{".duration": "10"}
	got, err := secondsFromExpr("@duration", schema, values)
	require.NoError(t, err)
	assert.Equal(t, float64(10), got)
}

func TestCharsFromExpr_FlatFieldNameReference(t *testing.T) {
	schema := []rhparser.SchemaParam{
		{FieldName: "prompt", Label: "提示词", Type: "textarea"},
	}
	values := map[string]any{".prompt": "你好世界"}
	got, err := charsFromExpr("len(@prompt)", schema, values)
	require.NoError(t, err)
	assert.Equal(t, float64(4), got)
}

// A bare numeric token must keep its literal meaning on flat schemas: no flat
// field is named after the number, so "10*2+1" stays arithmetic.
func TestSecondsFromExpr_LiteralArithmeticUnchangedOnFlatSchema(t *testing.T) {
	schema := []rhparser.SchemaParam{
		{FieldName: "duration", Label: "时长", Type: "number"},
	}
	values := map[string]any{".duration": "10"}
	got, err := secondsFromExpr("10*2+1", schema, values)
	require.NoError(t, err)
	assert.Equal(t, float64(21), got)
}
