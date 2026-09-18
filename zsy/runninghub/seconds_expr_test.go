package runninghub

import (
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The per-second billing path reads its run length from an admin-configured
// expression over node ids. These tests pin the accepted forms, the literal
// fallback, the billing-safe clamping, and the rejection paths that keep a
// malformed expression from silently billing a fallback.

// secondsTestSchema mirrors an app whose run length is derived from two nodes
// (开始秒数 / 执行秒数), plus one node carrying two fields.
func secondsTestSchema() []rhparser.SchemaParam {
	return []rhparser.SchemaParam{
		{NodeID: "229", FieldName: "value", Label: "开始秒数", Type: "number"},
		{NodeID: "212", FieldName: "value", Label: "执行秒数", Type: "number"},
		{NodeID: "300", FieldName: "value", Label: "时", Type: "number"},
		{NodeID: "300", FieldName: "seconds", Label: "秒", Type: "number"},
	}
}

// secondsTestValues: 开始秒数 = 3, 执行秒数/结束 = 11, 所以 212-229 = 8 秒。
func secondsTestValues() map[string]any {
	return map[string]any{
		"229.value":   "3",
		"212.value":   "11",
		"300.seconds": "30",
	}
}

func TestSecondsFromExpr_AcceptedForms(t *testing.T) {
	schema := secondsTestSchema()

	cases := []struct {
		name string
		expr string
		want float64
	}{
		{"bare node id", "212", 11},
		{"explicit node id", "nodeId=212", 11},
		{"spaces around equals", "nodeId = 212", 11},
		{"at-sign shorthand", "@212", 11},
		{"subtraction of two nodes", "212-229", 8},
		{"explicit subtraction", "nodeId=212-nodeId=229", 8},
		{"field-qualified reference", "nodeId=300.seconds", 30},
		{"node id resolves to the submitted field", "300", 30},
		{"literal when no such node exists", "212-2", 9},
		{"multiplication and addition", "(212-229)*2 + 1.5", 17.5},
		{"literals only", "7", 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := secondsFromExpr(tc.expr, schema, secondsTestValues())
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSecondsFromExpr_ClampsToBillingBounds(t *testing.T) {
	schema := secondsTestSchema()

	cases := []struct {
		name   string
		expr   string
		values map[string]any
		want   float64
	}{
		{
			name:   "negative difference becomes one second",
			expr:   "229-212",
			values: secondsTestValues(),
			want:   1,
		},
		{
			name:   "zero result becomes one second",
			expr:   "212-229",
			values: map[string]any{"229.value": "5", "212.value": "5"},
			want:   1,
		},
		{
			name:   "oversized result saturates at the duration ceiling",
			expr:   "212*1000",
			values: secondsTestValues(),
			want:   3600,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := secondsFromExpr(tc.expr, schema, tc.values)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSecondsFromExpr_Rejects(t *testing.T) {
	schema := secondsTestSchema()

	cases := []struct {
		name     string
		expr     string
		values   map[string]any
		wantPart string
	}{
		{"unknown node reference", "nodeId=999", secondsTestValues(), "不可用"},
		{"unsubmitted node reference", "nodeId=229", map[string]any{"212.value": "11"}, "不可用"},
		{"division by zero", "212/0", secondsTestValues(), "除以 0"},
		{"dangling operator", "229-", secondsTestValues(), "缺少数字或字段引用"},
		{"two operands without operator", "229 212", secondsTestValues(), "无法解析"},
		{"unquoted field reference", "229.value", secondsTestValues(), "无法识别"},
		{"unbalanced parenthesis", "(229-212", secondsTestValues(), "右括号"},
		{"illegal character", "229|212", secondsTestValues(), "非法字符"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := secondsFromExpr(tc.expr, schema, tc.values)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantPart)
		})
	}
}

func TestValidateSecondsExpr_SyntaxOnly(t *testing.T) {
	require.NoError(t, validateSecondsExpr("nodeId=229-nodeId=212"))
	require.NoError(t, validateSecondsExpr("(229-212)*2"))
	// References are resolved at submit time, so an unknown node id is not a
	// syntax error for the admin save path.
	require.NoError(t, validateSecondsExpr("nodeId=999"))

	require.Error(t, validateSecondsExpr(""))
	require.Error(t, validateSecondsExpr("229-"))
	require.Error(t, validateSecondsExpr("nodeId="))
	require.Error(t, validateSecondsExpr("abc"))
}

func TestResolveAppSeconds_PrefersConfiguredExpression(t *testing.T) {
	schema := secondsTestSchema()
	values := secondsTestValues()

	// An expression overrides the legacy duration/seconds-typed scan.
	app := &AppView{PerSecondBilling: true, SecondsExpr: "212-229"}
	got, err := resolveAppSeconds(app, schema, values)
	require.NoError(t, err)
	assert.Equal(t, 8.0, got)

	// Without an expression the legacy scan applies; this schema has no
	// seconds/duration-typed parameter, so the whole run falls back to one
	// second — the mis-pricing SecondsExpr exists to fix.
	got, err = resolveAppSeconds(&AppView{PerSecondBilling: true}, schema, values)
	require.NoError(t, err)
	assert.Equal(t, 1.0, got)

	typed := []rhparser.SchemaParam{{NodeID: "5", FieldName: "duration", Type: "seconds"}}
	got, err = resolveAppSeconds(&AppView{PerSecondBilling: true}, typed, map[string]any{"5.duration": "12"})
	require.NoError(t, err)
	assert.Equal(t, 12.0, got)
}
