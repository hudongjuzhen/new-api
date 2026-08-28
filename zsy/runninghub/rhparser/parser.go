// Package rhparser transforms raw inputs (an RH curl command, a JSON workflow
// export) into the canonical schema / submit shape used by the rest of the
// runninghub plugin. The package deliberately imports nothing from the main
// new-api module so it stays buildable in isolation (critical for the TDD
// step where the rest of the host process may not yet be wired).
package rhparser

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// NodeInfo is the shared shape produced by the curl parser and consumed by the
// upstream request builder. FieldValue is kept as string per §3.4: explicit
// zeroes and numeric precision strings (e.g. "1.0000000000000002") pass
// through untouched.
type NodeInfo struct {
	NodeID         string `json:"nodeId,omitempty"`
	FieldName      string `json:"fieldName,omitempty"`
	Field          string `json:"field,omitempty"`
	FieldValue     string `json:"fieldValue,omitempty"`
	Description    string `json:"description,omitempty"`
	DescriptionEn  string `json:"descriptionEn,omitempty"`
}

// ParsedCurl is the normalised output of ParseCurl.
type ParsedCurl struct {
	// Kind is one of ai_app / workflow / model.
	Kind string `json:"kind"`

	// UpstreamID is the id embedded in the submit path, or the model API's
	// relative path (model kind).
	UpstreamID string `json:"upstreamId"`

	// BaseURL is the scheme://host[:port] portion. Empty when the original
	// curl did not contain an absolute URL (e.g. shell variable used).
	BaseURL string `json:"baseUrl,omitempty"`

	// NodeInfoList contains parameters that the upstream request body listed.
	NodeInfoList []NodeInfo `json:"nodeInfoList,omitempty"`

	// RawBody is a copy of the parsed request body when the curl used a JSON
	// body directly (model APIs fall into this shape). Kept around so
	// follow-up validators can display the original value back to admins.
	RawBody json.RawMessage `json:"rawBody,omitempty"`
}

var (
	// reCurlURL extracts the first positional URL argument of curl. Accepts
	// both quoted (' / ") and unquoted forms.
	reCurlURL = regexp.MustCompile(`(?m)curl(?:\.exe)?[^\n]*?[\s'"]+(https?://[^\s'"]+)[\s'"]`)

	// reCurlMethod captures -X POST / -X GET. Defaults to POST when absent.
	reCurlMethod = regexp.MustCompile(`(?:^|\s)-X\s+([A-Z]+)`)

	// reCurlDataArg captures --data/-d <data>. Matches '...' , "..." , or
	// @file-syntax.
	reCurlDataArg = regexp.MustCompile(`(?:^|\s)(?:--data\b|-d\b)\s+(?:'([^']*)'|"([^"]*)"|(\S+))`)

	// reAICAppId extracts the id from /run/ai-app/<id>
	reAICAppId = regexp.MustCompile(`/run/ai-app/([A-Za-z0-9_-]+)`)

	// reWorkflowId extracts the id from /run/workflow/<id>
	reWorkflowId = regexp.MustCompile(`/run/workflow/([A-Za-z0-9_-]+)`)

	// reModelPath strips /openapi/v2/ (if present) and returns everything
	// following, interpreted as the model API's relative path.
	reModelPrefix = regexp.MustCompile(`/openapi/v2/(.+)$`)
)

// ParseCurl parses a single curl invocation and returns a normalised summary.
// It intentionally rejects inputs that do not look like a curl command at all
// (e.g. empty strings, plain URLs) so the admin UI can give a clear error.
func ParseCurl(raw string) (ParsedCurl, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedCurl{}, errors.New("empty curl input")
	}
	// Loose sanity check so we do not try to parse an arbitrary JSON blob as
	// curl. The leading line does not need to start with "curl" (it can start
	// with a shell continuation) but the text must contain the word curl.
	if !strings.Contains(strings.ToLower(raw), "curl") {
		return ParsedCurl{}, errors.New("input is not a curl command")
	}

	out := ParsedCurl{}

	// 1. URL
	m := reCurlURL.FindStringSubmatch(raw)
	if m == nil {
		return ParsedCurl{}, errors.New("no absolute http(s) URL found in curl")
	}
	rawURL := m[1]
	u, err := url.Parse(rawURL)
	if err != nil {
		return ParsedCurl{}, fmt.Errorf("invalid URL: %w", err)
	}
	out.BaseURL = u.Scheme + "://" + u.Host
	path := u.Path

	// 2. Kind / UpstreamID
	if mm := reAICAppId.FindStringSubmatch(path); mm != nil {
		out.Kind = "ai_app"
		out.UpstreamID = mm[1]
	} else if mm := reWorkflowId.FindStringSubmatch(path); mm != nil {
		out.Kind = "workflow"
		out.UpstreamID = mm[1]
	} else if mm := reModelPrefix.FindStringSubmatch(path); mm != nil {
		out.Kind = "model"
		out.UpstreamID = strings.TrimPrefix(mm[1], "/")
	} else {
		return ParsedCurl{}, fmt.Errorf("unrecognised RH endpoint: %s", path)
	}

	// 3. Request JSON body.
	bodyText, err := extractCurlData(raw)
	if err != nil {
		return ParsedCurl{}, err
	}
	if bodyText != "" {
		if !json.Valid([]byte(bodyText)) {
			return ParsedCurl{}, fmt.Errorf("curl body is not valid JSON: %s", truncate(bodyText, 80))
		}
		out.RawBody = json.RawMessage(bodyText)
		list, err := extractNodeInfo([]byte(bodyText))
		if err != nil {
			return ParsedCurl{}, err
		}
		out.NodeInfoList = list
	}
	return out, nil
}

// extractCurlData pulls the value of the first -d/--data argument. Multiline
// quoted bodies are handled by the regex already; the main edge case handled
// here is shell line continuations ('\') that people paste from RH docs.
func extractCurlData(raw string) (string, error) {
	// Normalise: drop shell backslash line continuations.
	clean := regexp.MustCompile(`\\\r?\n`).ReplaceAllString(raw, " ")
	m := reCurlDataArg.FindStringSubmatch(clean)
	if m == nil {
		return "", nil
	}
	// Three capture groups correspond to '...'  "..."  and bareword branches.
	for i := 1; i <= 3; i++ {
		if m[i] != "" {
			if strings.HasPrefix(m[i], "@") {
				return "", fmt.Errorf("curl uses @file syntax for body (%s); please paste the JSON content directly", truncate(m[i], 40))
			}
			return m[i], nil
		}
	}
	return "", nil
}

// extractNodeInfo decodes a submit JSON body into the canonical NodeInfo list.
// Three upstream shapes are supported:
//
//  1. V2 nodeInfoList form (AI 应用 / 工作流):
//     {"nodeInfoList": [{"nodeId":"122","fieldName":"prompt","fieldValue":"..."}]}
//  2. V1 webapp form (deprecated):
//     {"webappId":"...", "nodeInfoList": [...]}
//  3. Model API form (flat):
//     {"text":"...","voice_id":"...","speed":1.0}
//
// For case (3) each top-level key becomes a NodeInfo with NodeID empty and
// FieldName == Field == key.
func extractNodeInfo(body []byte) ([]NodeInfo, error) {
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return nil, fmt.Errorf("body JSON decode failed: %w", err)
	}
	if raw, ok := generic["nodeInfoList"]; ok && raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("nodeInfoList is not an array")
		}
		out := make([]NodeInfo, 0, len(list))
		for i, v := range list {
			entry, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("nodeInfoList[%d]: expected object, got %T", i, v)
			}
			n := NodeInfo{
				NodeID:        asString(entry["nodeId"]),
				FieldName:     asString(entry["fieldName"]),
				Field:         asString(entry["field"]),
				FieldValue:    asString(entry["fieldValue"]),
				Description:   asString(entry["description"]),
				DescriptionEn: asString(entry["descriptionEn"]),
			}
			out = append(out, n)
		}
		return out, nil
	}
	// Case 3: flat model API body.
	out := make([]NodeInfo, 0, len(generic))
	for k, v := range generic {
		if isIgnoredCurlTopLevelKey(k) {
			continue
		}
		s, err := asStringPreserveNumbers(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		out = append(out, NodeInfo{
			FieldName:  k,
			Field:      k,
			FieldValue: s,
		})
	}
	return out, nil
}

// isIgnoredCurlTopLevelKey drops structural fields present in the upstream
// body that are not end-user-editable parameters. Keeping them would produce
// misleading form fields in the admin UI.
func isIgnoredCurlTopLevelKey(k string) bool {
	switch k {
	case "instanceType", "usePersonalQueue", "webappId", "apiKey",
		"webhookUrl", "accessPassword", "workflowId", "clientId":
		return true
	}
	return false
}

// asString converts a JSON value to the string representation we store in
// NodeInfo. Numbers/bools pass through their standard JSON string
// representations so "false" and "0" stay distinguishable from absent values.
func asString(v any) string {
	s, err := asStringPreserveNumbers(v)
	if err != nil {
		return ""
	}
	return s
}

// asStringPreserveNumbers behaves like asString but returns a descriptive
// error for deeply nested values we cannot round-trip.
func asStringPreserveNumbers(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	switch t := v.(type) {
	case string:
		return t, nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case float64:
		// Use JSON marshal to keep "1" instead of "1.0" for integral floats.
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case json.Number:
		return t.String(), nil
	default:
		// Arrays / objects / nil maps are allowed only if empty; otherwise
		// return an error so the caller can surface it.
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("unsupported value type %T: %w", v, err)
		}
		s := string(b)
		if s == "{}" || s == "[]" || s == "null" {
			return "", nil
		}
		return "", fmt.Errorf("unsupported value (object/array); expected scalar: %s", truncate(s, 40))
	}
}

// truncate returns s capped at n runes with "…" appended when truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// --- Validation / schema build for the admin UI ----------------------------

// ErrSchemaReport lists per-field errors produced by Validate. Keeping the
// report structured (rather than a joined string) lets the admin UI render
// markers next to the corresponding form row.
type ErrSchemaReport struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// SchemaSummary is the validated & enriched result of BuildSchemaFromNodes.
type SchemaSummary struct {
	Params []SchemaParam `json:"params"`
	Errors []ErrSchemaReport `json:"errors,omitempty"`
}

// SchemaParam mirrors the plugin-level FieldParam but lives in rhparser so it
// can be used from pure tests without pulling the full runninghub package.
type SchemaParam struct {
	NodeID      string                `json:"nodeId"`
	FieldName   string                `json:"fieldName"`
	Label       string                `json:"label"`
	Type        string                `json:"type"` // text / textarea / number / image / audio / video / select
	Required    bool                  `json:"required"`
	Default     string                `json:"defaultValue,omitempty"`
	Placeholder string                `json:"placeholder,omitempty"`
	Min         *float64              `json:"min,omitempty"`
	Max         *float64              `json:"max,omitempty"`
	Options     []SchemaParamOption   `json:"options,omitempty"`
}

// SchemaParamOption is one enum entry for Type == "select".
type SchemaParamOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// BuildSchemaFromNodes turns a raw node info list (from a parsed curl, or
// from a freshly imported app) into a draft schema suitable for saving. The
// heuristic is conservative: fields are text by default, image/audio/video
// inferred from filename patterns or select defaults. The caller is expected
// to tweak the draft in the admin UI.
func BuildSchemaFromNodes(nodes []NodeInfo) SchemaSummary {
	out := SchemaSummary{
		Params: make([]SchemaParam, 0, len(nodes)),
	}
	seen := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		key := n.NodeID + "##" + n.FieldName
		if _, dup := seen[key]; dup {
			out.Errors = append(out.Errors, ErrSchemaReport{
				Field:   n.FieldName,
				Message: fmt.Sprintf("duplicate nodeId=%s fieldName=%s", n.NodeID, n.FieldName),
			})
			continue
		}
		seen[key] = struct{}{}
		if n.FieldName == "" {
			out.Errors = append(out.Errors, ErrSchemaReport{
				Field:   "",
				Message: fmt.Sprintf("nodeId=%s: empty fieldName", n.NodeID),
			})
			continue
		}
		param := SchemaParam{
			NodeID:    n.NodeID,
			FieldName: n.FieldName,
			Label:     labelFromFieldName(n.FieldName, n.Description, n.DescriptionEn),
			Type:      inferParamType(n),
			Default:   n.FieldValue,
			Required:  true,
		}
		if param.Type == "number" {
			if min, max, ok := inferRangeHint(n.FieldValue); ok {
				param.Min = &min
				param.Max = &max
			}
		}
		out.Params = append(out.Params, param)
	}
	return out
}

// labelFromFieldName returns a human-readable label for a parameter. The
// heuristics prefer the explicit description fields (admin can still
// override); fall back to snake_case → Title Case transformation.
func labelFromFieldName(field, desc, descEn string) string {
	if desc != "" {
		return desc
	}
	if descEn != "" {
		return descEn
	}
	return humanizeSnake(field)
}

func humanizeSnake(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		r := []rune(p)
		if len(r) == 0 {
			continue
		}
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// inferParamType inspects a node entry and returns a best-effort param type.
func inferParamType(n NodeInfo) string {
	name := strings.ToLower(n.FieldName)
	value := strings.ToLower(n.FieldValue)
	// URL-only fields are assumed to be media uploads.
	switch {
	case strings.Contains(name, "image") || strings.Contains(name, "img") || strings.HasSuffix(name, "_pic"):
		return "image"
	case strings.Contains(name, "video") || strings.HasSuffix(name, "_mp4"):
		return "video"
	case strings.Contains(name, "audio") || strings.Contains(name, "voice") || strings.Contains(name, "speech"):
		return "audio"
	}
	// Ending with common image extensions also implies image type, the
	// default value is just a pre-filled uploaded fileName.
	switch {
	case strings.HasSuffix(value, ".png"), strings.HasSuffix(value, ".jpg"),
		strings.HasSuffix(value, ".jpeg"), strings.HasSuffix(value, ".webp"):
		return "image"
	case strings.HasSuffix(value, ".mp4"), strings.HasSuffix(value, ".mov"),
		strings.HasSuffix(value, ".webm"):
		return "video"
	case strings.HasSuffix(value, ".mp3"), strings.HasSuffix(value, ".wav"),
		strings.HasSuffix(value, ".flac"):
		return "audio"
	}
	switch {
	case looksNumeric(n.FieldValue):
		return "number"
	case len(n.FieldValue) > 40 || strings.ContainsAny(n.FieldValue, "\n\r"):
		return "textarea"
	}
	return "text"
}

// looksNumeric reports whether s parses as a finite number.
func looksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// inferRangeHint returns a (min,max) pair for obvious enumerations. When the
// field value is a plain number and no context exists, ok is false (caller
// will not pin a range).
func inferRangeHint(value string) (min, max float64, ok bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, 0, false
	}
	// Common known heuristics.
	switch value {
	case "0", "1":
		return 0, 1, true
	}
	switch {
	case v >= 0.25 && v <= 4.0:
		// audio / prompt weights.
		return 0.25, 4.0, true
	case v > 0 && v <= 1.0:
		return 0, 1, true
	}
	return 0, 0, false
}
