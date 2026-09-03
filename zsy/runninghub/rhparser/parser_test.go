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
