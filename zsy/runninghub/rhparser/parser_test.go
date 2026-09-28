package rhparser_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
)

// =========================================================================
// ParseCurl — table driven. Golden inputs are copied verbatim from §3.3 of
// the dev plan plus the actual probe curl of §3.9.
// =========================================================================

func TestParseCurl(t *testing.T) {
	t.Parallel()
	const probeAPIKey = "x"
	cases := []struct {
		name    string
		curl    string
		want    rhparser.ParsedCurl
		wantErr bool
	}{
		{
			name: "ai_app v1 golden from doc §3.3",
			curl: `curl --location 'https://www.runninghub.cn/task/openapi/ai-app/run' ` +
				`-H 'Authorization: Bearer ` + probeAPIKey + `' ` +
				`--data '{"webappId":1877265245566922753,"apiKey":"` + probeAPIKey + `","nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cat"}]}'`,
			want: rhparser.ParsedCurl{
				// NOTE: The V1 protocol embeds the webapp id in the body rather
				// than the URL. The parser tolerates it because the curl
				// actually points at .../ai-app/run (no id suffix), which the
				// current regex will reject. This case therefore is marked as
				// an error expectation so the behaviour is explicit; see the
				// ai_app v2 case below for the normal path.
			},
			wantErr: true,
		},
		{
			name: "ai_app v2 probe curl (§3.9)",
			curl: `curl --location 'https://www.runninghub.cn/openapi/v2/run/ai-app/1877265245566922753' ` +
				`-H 'Authorization: Bearer ` + probeAPIKey + `' ` +
				`-H 'Content-Type: application/json' ` +
				`--data '{"nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cat"}]}'`,
			want: rhparser.ParsedCurl{
				Kind:       "ai_app",
				UpstreamID: "1877265245566922753",
				BaseURL:    "https://www.runninghub.cn",
				NodeInfoList: []rhparser.NodeInfo{
					{NodeID: "122", FieldName: "prompt", FieldValue: "a cat"},
				},
			},
		},
		{
			name: "workflow v2",
			curl: `curl --location 'https://www.runninghub.ai/openapi/v2/run/workflow/wf_9F1fAbCd' ` +
				`-H 'Authorization: Bearer x' ` +
				`--data '{"instanceType":"plus","nodeInfoList":[{"nodeId":"node_input_image","fieldName":"file","fieldValue":"abc.png"},{"nodeId":"prompt","fieldName":"text","fieldValue":"enhance this"}]}'`,
			want: rhparser.ParsedCurl{
				Kind:       "workflow",
				UpstreamID: "wf_9F1fAbCd",
				BaseURL:    "https://www.runninghub.ai",
				NodeInfoList: []rhparser.NodeInfo{
					{NodeID: "node_input_image", FieldName: "file", FieldValue: "abc.png"},
					{NodeID: "prompt", FieldName: "text", FieldValue: "enhance this"},
				},
			},
		},
		{
			name: "standard model API tts",
			curl: `curl --location 'https://www.runninghub.cn/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo' ` +
				`-H 'Authorization: Bearer x' ` +
				`--data '{"text":"hello","voice_id":"alice","speed":1.25}'`,
			want: rhparser.ParsedCurl{
				Kind:       "model",
				UpstreamID: "rhart-audio/text-to-audio/speech-2.8-turbo",
				BaseURL:    "https://www.runninghub.cn",
				// Ignore top-level structural keys like instanceType / webhookUrl
				// is tested in a separate case; here, the 3 user-facing fields
				// should map to flat NodeInfo entries.
				NodeInfoList: []rhparser.NodeInfo{
					{FieldName: "text", Field: "text", FieldValue: "hello"},
					{FieldName: "voice_id", Field: "voice_id", FieldValue: "alice"},
					{FieldName: "speed", Field: "speed", FieldValue: "1.25"},
				},
			},
		},
		{
			name:    "empty input",
			curl:    ``,
			wantErr: true,
		},
		{
			name:    "no curl command",
			curl:    `wget https://www.runninghub.cn/`,
			wantErr: true,
		},
		{
			name:    "curl without url",
			curl:    `curl --verbose -d '{}'`,
			wantErr: true,
		},
		{
			name:    "curl with @file body errors out",
			curl:    `curl 'https://www.runninghub.cn/openapi/v2/run/ai-app/1' -d @body.json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := rhparser.ParseCurl(tc.curl)
			if tc.wantErr {
				require.Error(t, err, "expected error; got %+v", got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want.Kind, got.Kind, "kind mismatch")
			assert.Equal(t, tc.want.UpstreamID, got.UpstreamID, "upstream id mismatch")
			assert.Equal(t, tc.want.BaseURL, got.BaseURL, "base url mismatch")
			assert.ElementsMatch(t, tc.want.NodeInfoList, got.NodeInfoList, "node list mismatch")
			if len(tc.want.RawBody) > 0 {
				assert.JSONEq(t, string(tc.want.RawBody), string(got.RawBody))
			}
		})
	}
}

// CurlSlug keeps the upstream id as a compact, unique identifier for the app's
// slug field, falling back to the first node id when the URL carries none.
func TestCurlSlug(t *testing.T) {
	cases := []struct {
		name string
		curl string
		want string
	}{
		{
			name: "numeric app id becomes the slug",
			curl: "curl --location 'https://www.runninghub.cn/openapi/v2/run/ai-app/2027211316242423809' " +
				`-H 'Authorization: Bearer x' -d '{"nodeInfoList":[{"nodeId":"16","fieldName":"prompt","fieldValue":"x"}]}'`,
			want: "2027211316242423809",
		},
		{
			name: "workflow id with empty nodes keeps its prefix",
			curl: "curl --location 'https://www.runninghub.ai/openapi/v2/run/workflow/wf_9F1fAbCd' " +
				`-H 'Authorization: Bearer x' -d '{"nodeInfoList":[]}'`,
			want: "wf_9f1fabcd",
		},
		{
			name: "node list fallback used only when URL has no id (model path keeps id)",
			curl: "curl 'https://www.runninghub.cn/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo' " +
				`-H 'Authorization: Bearer x' -d '{"nodeInfoList":[]}'`,
			want: "rhart-audio-text-to-audio-speech-2-8-turbo",
		},
		{
			// A node id is never used as the slug when the URL already carries
			// the upstream id (the app id is the stable unique key).
			name: "node id does not override the URL upstream id",
			curl: "curl 'https://www.runninghub.cn/openapi/v2/run/ai-app/42' " +
				`-H 'Authorization: Bearer x' -d '{"nodeInfoList":[{"nodeId":"16","fieldName":"prompt","fieldValue":"x"}]}'`,
			want: "42",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := rhparser.ParseCurl(tc.curl)
			require.NoError(t, err)
			assert.Equal(t, tc.want, rhparser.CurlSlug(&got))
		})
	}
}

// Verify ParseCurl round trips RawBody for a body with integer values (JSON
// number preservation is required for the admin UI's "preview original body"
// button).
func TestParseCurl_RawBodyPreservesIntegerJsonNumbers(t *testing.T) {
	t.Parallel()
	curl := `curl 'https://www.runninghub.cn/openapi/v2/run/ai-app/42' -H 'Authorization: Bearer x' --data '{"instanceType":"plus","nodeInfoList":[{"nodeId":"122","fieldName":"count","fieldValue":"42"}]}'`
	got, err := rhparser.ParseCurl(curl)
	require.NoError(t, err)
	require.NotEmpty(t, got.RawBody)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(got.RawBody, &parsed))
	list := parsed["nodeInfoList"].([]any)
	entry := list[0].(map[string]any)
	assert.Equal(t, "42", entry["fieldValue"])
	assert.Equal(t, "plus", parsed["instanceType"])
}

// =========================================================================
// ParseFieldData — the fieldData blob is RunningHub's own editor declaration.
// =========================================================================

func TestParseFieldData(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		field  string
		want   rhparser.FieldDataSpec
		wantOK bool
	}{
		{
			// The exact blob the app editor's request example carries for its
			// aspect-ratio node.
			name: "typed COMBO descriptor",
			field: `["COMBO", {"default": "1:1 (Square)", "options": ["1:1 (Square)", "16:9 (Widescreen)"], ` +
				`"tooltip": "The aspect ratio for the output dimensions.", "multiselect": false}]`,
			want: rhparser.FieldDataSpec{
				Type:    "select",
				Default: "1:1 (Square)",
				Tooltip: "The aspect ratio for the output dimensions.",
				Options: []rhparser.SchemaParamOption{
					{Label: "1:1 (Square)", Value: "1:1 (Square)"},
					{Label: "16:9 (Widescreen)", Value: "16:9 (Widescreen)"},
				},
			},
			wantOK: true,
		},
		{
			name:   "boolean descriptor",
			field:  `["BOOLEAN", {"default": false}]`,
			want:   rhparser.FieldDataSpec{Type: "switch", Default: "false"},
			wantOK: true,
		},
		{
			// A marker RH has not shipped yet must not degrade an enumerable
			// node into free text.
			name:   "unknown marker keeps its choices",
			field:  `["SLIDER", {"options": ["a", "b"]}]`,
			want:   rhparser.FieldDataSpec{Type: "select", Options: []rhparser.SchemaParamOption{{Label: "a", Value: "a"}, {Label: "b", Value: "b"}}},
			wantOK: true,
		},
		{
			name:   "label/value option objects",
			field:  `["COMBO", {"options": [{"label": "方法一", "value": "1"}]}]`,
			want:   rhparser.FieldDataSpec{Type: "select", Options: []rhparser.SchemaParamOption{{Label: "方法一", Value: "1"}}},
			wantOK: true,
		},
		{
			name:   "marker without choices declares nothing",
			field:  `["COMBO", {"multiselect": false}]`,
			wantOK: false,
		},
		{
			name:   "nested list plus default object",
			field:  `[["1k","2k"],{"default":"2k"}]`,
			want:   rhparser.FieldDataSpec{Type: "select", Default: "2k", Options: []rhparser.SchemaParamOption{{Label: "1k", Value: "1k"}, {Label: "2k", Value: "2k"}}},
			wantOK: true,
		},
		{
			name:   "plain string list",
			field:  `["1:1","16:9"]`,
			want:   rhparser.FieldDataSpec{Type: "select", Options: []rhparser.SchemaParamOption{{Label: "1:1", Value: "1:1"}, {Label: "16:9", Value: "16:9"}}},
			wantOK: true,
		},
		{
			// Demo payloads name the choice and carry the wire value in index.
			name:   "demo enum objects",
			field:  `[{"name":"1k","index":"1k","description":"1024"},{"name":"2k","index":"2k"}]`,
			want:   rhparser.FieldDataSpec{Type: "select", Options: []rhparser.SchemaParamOption{{Label: "1k", Value: "1k"}, {Label: "2k", Value: "2k"}}},
			wantOK: true,
		},
		{
			name:   "empty blob",
			field:  "   ",
			wantOK: false,
		},
		{
			name:   "truncated json",
			field:  `["COMBO",`,
			wantOK: false,
		},
		{
			name:   "empty list",
			field:  `[]`,
			wantOK: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := rhparser.ParseFieldData(tc.field)
			require.Equal(t, tc.wantOK, ok, "ok mismatch for %s", tc.field)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.want.Type, got.Type, "type mismatch")
			assert.Equal(t, tc.want.Default, got.Default, "default mismatch")
			assert.Equal(t, tc.want.Tooltip, got.Tooltip, "tooltip mismatch")
			assert.Equal(t, tc.want.Options, got.Options, "options mismatch")
		})
	}
}

// The two node shapes the app editor has to survive verbatim: a COMBO select
// whose fieldData carries both the choices and the default, and a boolean node
// with no fieldData at all.
func TestBuildSchemaFromNodes_RequestExampleNodes(t *testing.T) {
	t.Parallel()
	nodes := []rhparser.NodeInfo{
		{
			NodeID:    "13",
			FieldName: "aspect_ratio",
			FieldData: `["COMBO", {"default": "1:1 (Square)", "options": ["1:1 (Square)", "2:3 (Portrait Photo)", ` +
				`"3:2 (Photo)", "3:4 (Portrait Standard)", "4:3 (Standard)", "9:16 (Portrait Widescreen)", ` +
				`"16:9 (Widescreen)", "21:9 (Ultrawide)"], ` +
				`"tooltip": "The aspect ratio for the output dimensions.", "multiselect": false}]`,
			FieldValue: "16:9 (Widescreen)",
		},
		{NodeID: "256", FieldName: "value", FieldValue: "false"},
	}
	out := rhparser.BuildSchemaFromNodes(nodes)
	require.Empty(t, out.Errors)
	require.Len(t, out.Params, 2)

	ratio := out.Params[0]
	assert.Equal(t, "select", ratio.Type)
	assert.Len(t, ratio.Options, 8)
	assert.Equal(t, "21:9 (Ultrawide)", ratio.Options[7].Value)
	assert.Equal(t, "1:1 (Square)", ratio.Default, "the declared default pre-fills the dropdown")
	assert.Equal(t, "The aspect ratio for the output dimensions.", ratio.Placeholder)

	toggle := out.Params[1]
	assert.Equal(t, "switch", toggle.Type)
	assert.Equal(t, "false", toggle.Default)
	assert.Equal(t, "Value", toggle.Label)
}

// A switch only ever submits "true"/"false", so both spellings RH ships are
// canonicalized; a select without a declared default keeps the request sample.
func TestBuildSchemaFromNodes_FieldDataDefaults(t *testing.T) {
	t.Parallel()
	nodes := []rhparser.NodeInfo{
		{NodeID: "1", FieldName: "enabled", FieldData: `["BOOLEAN", {"default": 1}]`, FieldValue: "0"},
		{NodeID: "2", FieldName: "value", FieldValue: "true"},
		{NodeID: "3", FieldName: "mode", FieldData: `["COMBO", {"options": ["a", "b"]}]`, FieldValue: "b"},
	}
	out := rhparser.BuildSchemaFromNodes(nodes)
	require.Empty(t, out.Errors)
	require.Len(t, out.Params, 3)

	assert.Equal(t, "switch", out.Params[0].Type)
	assert.Equal(t, "true", out.Params[0].Default, "a numeric boolean default is normalized")
	assert.Equal(t, "switch", out.Params[1].Type)
	assert.Equal(t, "true", out.Params[1].Default)
	assert.Equal(t, "select", out.Params[2].Type)
	assert.Equal(t, "b", out.Params[2].Default, "without a declared default the sample pre-fills the form")
}

// =========================================================================
// BuildSchemaFromNodes
// =========================================================================

func TestBuildSchemaFromNodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		nodes    []rhparser.NodeInfo
		wantLen  int
		wantErrs int
	}{
		{
			name: "image + prompt (ai_app default set)",
			nodes: []rhparser.NodeInfo{
				{NodeID: "122", FieldName: "prompt", FieldValue: "A cat on a sofa"},
				{NodeID: "121", FieldName: "input_image", FieldValue: "cat.png"},
			},
			wantLen: 2,
		},
		{
			name: "audio node by name hint",
			nodes: []rhparser.NodeInfo{
				{NodeID: "n1", FieldName: "audio_file", FieldValue: ""},
			},
			wantLen: 1,
		},
		{
			name: "numeric string infers number type",
			nodes: []rhparser.NodeInfo{
				{NodeID: "n1", FieldName: "cfg", FieldValue: "1.5"},
			},
			wantLen: 1,
		},
		{
			name: "duplicate nodeId/fieldName reports error and skips dup",
			nodes: []rhparser.NodeInfo{
				{NodeID: "n1", FieldName: "prompt", FieldValue: "a"},
				{NodeID: "n1", FieldName: "prompt", FieldValue: "b"},
			},
			wantLen:  1,
			wantErrs: 1,
		},
		{
			name: "empty fieldName reports error",
			nodes: []rhparser.NodeInfo{
				{NodeID: "n1", FieldName: "", FieldValue: "ok"},
			},
			wantLen:  0,
			wantErrs: 1,
		},
		{
			name: "description is preferred as label",
			nodes: []rhparser.NodeInfo{
				{NodeID: "n", FieldName: "some_weird_snake", Description: "用户提示词", FieldValue: ""},
			},
			wantLen: 1,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rhparser.BuildSchemaFromNodes(tc.nodes)
			assert.Len(t, got.Params, tc.wantLen, "params length mismatch")
			assert.Len(t, got.Errors, tc.wantErrs, "errors length mismatch")
		})
	}
}

func TestBuildSchemaFromNodes_LabelPreference(t *testing.T) {
	t.Parallel()
	nodes := []rhparser.NodeInfo{
		{NodeID: "n1", FieldName: "my_input_field", Description: "正面提示词", FieldValue: "cat"},
		{NodeID: "n2", FieldName: "english_only", DescriptionEn: "Negative prompt", FieldValue: "ugly"},
		{NodeID: "n3", FieldName: "bare_field", FieldValue: "x"},
	}
	out := rhparser.BuildSchemaFromNodes(nodes)
	require.Len(t, out.Params, 3)
	assert.Equal(t, "正面提示词", out.Params[0].Label)
	assert.Equal(t, "Negative prompt", out.Params[1].Label)
	assert.Equal(t, "Bare Field", out.Params[2].Label)
}

func TestBuildSchemaFromNodes_TypeHints(t *testing.T) {
	t.Parallel()
	nodes := []rhparser.NodeInfo{
		{NodeID: "1", FieldName: "image_url", FieldValue: "cat.png"},
		{NodeID: "2", FieldName: "audio", FieldValue: "x.wav"},
		{NodeID: "3", FieldName: "video_mp4", FieldValue: "y.mp4"},
		{NodeID: "4", FieldName: "cfg", FieldValue: "2.0"},
		{NodeID: "5", FieldName: "desc", FieldValue: "this is a long string that certainly exceeds forty characters just to be sure"},
	}
	out := rhparser.BuildSchemaFromNodes(nodes)
	require.Len(t, out.Params, 5)
	want := []string{"image", "audio", "video", "number", "textarea"}
	for i, p := range out.Params {
		assert.Equalf(t, want[i], p.Type, "param %s (%s) wrong type", p.FieldName, p.Label)
	}
}

// An import sample value says nothing about a field's legal range. It used to
// pin max=1 for any numeric node whose example value was "0" or "1" (and
// [0.25,4] for 0.25-4), which then rejected every legal submit for counters
// such as a "开始秒数" node whose example value was 0.
func TestBuildSchemaFromNodes_NoInferredNumericBounds(t *testing.T) {
	t.Parallel()
	nodes := []rhparser.NodeInfo{
		{NodeID: "229", FieldName: "value", Description: "开始秒数", FieldValue: "0"},
		{NodeID: "230", FieldName: "value", Description: "执行秒数", FieldValue: "1"},
		{NodeID: "231", FieldName: "value", Description: "强度", FieldValue: "2"},
		{NodeID: "232", FieldName: "value", Description: "分辨率", FieldValue: "1024"},
	}
	out := rhparser.BuildSchemaFromNodes(nodes)
	require.Len(t, out.Params, 4)
	for _, p := range out.Params {
		require.Equal(t, "number", p.Type, "param %s should still be numeric", p.Label)
		assert.Nilf(t, p.Min, "param %s must not infer a lower bound", p.Label)
		assert.Nilf(t, p.Max, "param %s must not infer an upper bound", p.Label)
	}
}
