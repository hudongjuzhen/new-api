package tone

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// The Tone Plaza CSV contract ------------------------------------------------
//
// Export writes a canonical English header; import accepts that header plus the
// Chinese spellings an operator is likely to type, so a spreadsheet filled in by
// hand imports without renaming columns. Everything about the round trip lives
// here: the same code writes an export and feeds an import.
//
// This file is also the **authoring path for the 文风标准** (see standard.go).
// The seeded catalog an operator starts from is a CSV, so the column order below
// is the standard's field order and the aliases below are the spellings the
// standard tolerates.

// CSV bounds. An import is an admin action, so the caps exist to bound memory
// and work rather than to be generous.
const (
	maxImportBytes = 5 << 20
	maxImportRows  = 5000

	// maxReportedRowErrors caps the per-row error list in one response; the
	// `failed` counter still reports every rejected row.
	maxReportedRowErrors = 200
)

// Import modes.
const (
	// ImportModeUpsert creates unknown names and updates the tones already
	// stored under a name from the file.
	ImportModeUpsert = "upsert"
	// ImportModeCreate only creates; an existing name is reported as a failed row
	// so a merge never happens by accident.
	ImportModeCreate = "create"
)

// csvColumn is one exported column: the canonical header written by the export
// plus the header spellings accepted on import.
type csvColumn struct {
	key     string
	aliases []string
}

// csvColumns fixes the export column order and the accepted import headers. The
// required pair (name, prompt) is validated by ParseTonesCSV. Prompt sits right
// after the introduction because it is the payload — a spreadsheet's third
// column is where a reader looks for "what this style actually says".
var csvColumns = []csvColumn{
	{key: "name", aliases: []string{"文风名称", "名称", "文风名", "风格名称"}},
	{key: "description", aliases: []string{"简介", "介绍", "描述", "说明"}},
	{key: "prompt", aliases: []string{"提示词", "文风提示词", "写作指令", "指令", "提示词正文", "prompt正文"}},
	{key: "category", aliases: []string{"类别", "分类", "体裁", "文种"}},
	{key: "tone", aliases: []string{"语气", "语调", "情绪", "调性"}},
	{key: "language", aliases: []string{"语言", "语言代码"}},
	{key: "scenes", aliases: []string{"scene", "适合场景", "场景", "适用场景"}},
	{key: "sample_input", aliases: []string{"示例原文", "原文示例", "样例原文", "示例输入", "原样文"}},
	{key: "sample_output", aliases: []string{"示例改写", "改写示例", "样例改写", "示例输出", "改写后"}},
	{key: "enabled", aliases: []string{"是否上架", "上架", "启用"}},
	{key: "sort_order", aliases: []string{"排序", "排序值", "顺序"}},
}

// requiredCSVColumns are the columns an import cannot work without, with the
// label used in the error message.
//
// `prompt` is required, matching validateTone: a tone without its instruction
// would import as a name and a description that no consumer can send anywhere.
var requiredCSVColumns = []struct {
	key   string
	label string
}{
	{key: "name", label: "name（文风名称）"},
	{key: "prompt", label: "prompt（文风提示词）"},
}

// csvColumnIndex maps every accepted header spelling (normalized) to the
// canonical column key.
var csvColumnIndex = buildCSVColumnIndex()

func buildCSVColumnIndex() map[string]string {
	index := make(map[string]string, len(csvColumns)*4)
	for _, column := range csvColumns {
		index[normalizeCSVHeader(column.key)] = column.key
		for _, alias := range column.aliases {
			index[normalizeCSVHeader(alias)] = column.key
		}
	}
	return index
}

// normalizeCSVHeader lower-cases a header cell and drops the separators an
// operator may type ("Sample Input", "sample-input", a UTF-8 BOM left by Excel),
// so "Sample_Input" and "sample input" both reach the same column.
func normalizeCSVHeader(raw string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(trimmed))
}

// WriteTonesCSV writes the canonical header and one row per tone, prefixed with
// a UTF-8 BOM: without it Excel on Windows reads the file as the local code page
// and mangles every Chinese value.
func WriteTonesCSV(w io.Writer, rows []Tone) error {
	if _, err := io.WriteString(w, "\ufeff"); err != nil {
		return fmt.Errorf("写入 CSV 失败: %w", err)
	}
	writer := csv.NewWriter(w)

	header := make([]string, 0, len(csvColumns))
	for _, column := range csvColumns {
		header = append(header, column.key)
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	for i := range rows {
		if err := writer.Write(toneCSVRecord(&rows[i])); err != nil {
			return fmt.Errorf("写入 CSV 第 %d 行失败: %w", i+2, err)
		}
	}
	writer.Flush()
	return writer.Error()
}

// toneCSVRecord renders one stored tone in csvColumns order. Scenes go out as
// the canonical comma-separated string, which the CSV writer quotes, so an
// export re-imports unchanged.
func toneCSVRecord(t *Tone) []string {
	return []string{
		t.Name,
		t.Description,
		t.Prompt,
		t.Category,
		t.Tone,
		t.Language,
		t.Scenes,
		t.SampleInput,
		t.SampleOutput,
		strconv.FormatBool(t.Enabled),
		strconv.Itoa(t.SortOrder),
	}
}

// ToneImportRow is one parsed record: Create is the shape used when the name is
// free, Update the shape applied when it already exists. Update only carries the
// columns the file actually had, so an upsert never blanks a field the
// spreadsheet omitted.
type ToneImportRow struct {
	// Row is the record number in the uploaded file, counting the header as
	// row 1, so an error names the line the operator must fix.
	Row    int
	Create ToneCreateDTO
	Update ToneUpdateDTO
	// Err carries a row-local parse failure (an unreadable number or flag).
	// ImportTones reports it as a failed row instead of discarding the file.
	Err error
}

// ParseTonesCSV reads an uploaded CSV into rows. Structural problems — an empty
// file, a missing required column, an unreadable record, too many rows — fail
// the whole request, because guessing a column mapping would import the wrong
// data. Everything else is left to per-row validation.
func ParseTonesCSV(r io.Reader) ([]ToneImportRow, []string, error) {
	reader := csv.NewReader(r)
	// Ragged rows are tolerated: a short row simply leaves its tail cells
	// empty, which is what a hand-edited spreadsheet usually means.
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, errors.New("CSV 文件为空")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("读取 CSV 表头失败: %w", err)
	}

	index := make(map[string]int, len(header))
	unknown := make([]string, 0)
	for position, cell := range header {
		key, ok := csvColumnIndex[normalizeCSVHeader(cell)]
		if !ok {
			if trimmed := strings.TrimSpace(cell); trimmed != "" {
				unknown = append(unknown, trimmed)
			}
			continue
		}
		if _, duplicate := index[key]; !duplicate {
			index[key] = position
		}
	}
	for _, required := range requiredCSVColumns {
		if _, ok := index[required.key]; !ok {
			return nil, nil, fmt.Errorf("CSV 缺少必需列: %s", required.label)
		}
	}

	warnings := make([]string, 0, 2)
	if len(unknown) > 0 {
		warnings = append(warnings, "已忽略无法识别的列: "+strings.Join(unknown, ", "))
	}

	rows := make([]ToneImportRow, 0, 16)
	record := 1 // the header is record 1
	for {
		cells, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("读取 CSV 第 %d 行失败: %w", record+1, err)
		}
		record++
		if isBlankCSVRecord(cells) {
			continue
		}
		if len(rows) >= maxImportRows {
			return nil, nil, fmt.Errorf("CSV 行数超过上限 %d 行", maxImportRows)
		}
		row := parseToneCSVRow(cells, index)
		row.Row = record
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, warnings, errors.New("CSV 中没有可导入的数据行")
	}
	warnings = append(warnings, sampleInputWarnings(rows)...)
	return rows, warnings, nil
}

// isBlankCSVRecord reports whether every cell is empty, which is how a trailing
// blank line or an untouched spreadsheet row arrives.
func isBlankCSVRecord(cells []string) bool {
	for _, cell := range cells {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// sampleInputWarnings nudges a file towards the worked-example convention the
// standard describes: rows meant to be compared should share one SampleInput.
//
// It is a warning and never an error. Divergent examples are legitimate — an
// operator may be filing one tone at a time — and a standard that rejects the
// non-conforming case would just teach people to leave the column empty.
func sampleInputWarnings(rows []ToneImportRow) []string {
	seen := make(map[string]struct{}, len(rows))
	missing := 0
	for _, row := range rows {
		if row.Err != nil {
			continue
		}
		sample := strings.TrimSpace(row.Create.SampleInput)
		if sample == "" {
			missing++
			continue
		}
		seen[sample] = struct{}{}
	}
	out := make([]string, 0, 2)
	if len(seen) > 1 {
		out = append(out, fmt.Sprintf(
			"本文件的示例原文有 %d 种不同写法：标准建议多条文风共用同一段示例原文，"+
				"这样它们在广场上可以横向比较（各自的示例改写不同即可）", len(seen)))
	}
	if len(seen) > 0 && missing > 0 {
		out = append(out, fmt.Sprintf(
			"有 %d 行没有填示例原文：文风既不能试听也不能看脸，缺示例的条目在广场上只能凭名字选", missing))
	}
	return out
}

// parseToneCSVRow maps one record onto the create/update shapes. A cell that
// cannot be read (an unknown shelf flag, a non-numeric sort order) is recorded
// on the row instead of failing the file.
func parseToneCSVRow(cells []string, index map[string]int) ToneImportRow {
	row := ToneImportRow{}
	value := func(key string) (string, bool) {
		position, ok := index[key]
		if !ok || position >= len(cells) {
			return "", false
		}
		return strings.TrimSpace(cells[position]), true
	}

	if raw, ok := value("name"); ok {
		row.Create.Name, row.Update.Name = raw, &raw
	}
	if raw, ok := value("description"); ok {
		row.Create.Description, row.Update.Description = raw, &raw
	}
	if raw, ok := value("prompt"); ok {
		row.Create.Prompt, row.Update.Prompt = raw, &raw
	}
	if raw, ok := value("category"); ok {
		row.Create.Category, row.Update.Category = raw, &raw
	}
	if raw, ok := value("tone"); ok {
		row.Create.Tone, row.Update.Tone = raw, &raw
	}
	if raw, ok := value("language"); ok {
		row.Create.Language, row.Update.Language = raw, &raw
	}
	if raw, ok := value("scenes"); ok {
		scenes := ScenesInput(splitScenes(raw))
		row.Create.Scenes, row.Update.Scenes = scenes, &scenes
	}
	if raw, ok := value("sample_input"); ok {
		row.Create.SampleInput, row.Update.SampleInput = raw, &raw
	}
	if raw, ok := value("sample_output"); ok {
		row.Create.SampleOutput, row.Update.SampleOutput = raw, &raw
	}
	if raw, ok := value("enabled"); ok && raw != "" {
		enabled, err := parseCSVShelfFlag(raw)
		if err != nil {
			row.Err = err
			return row
		}
		row.Create.Enabled, row.Update.Enabled = &enabled, &enabled
	}
	if raw, ok := value("sort_order"); ok && raw != "" {
		sortOrder, err := strconv.Atoi(raw)
		if err != nil {
			row.Err = fmt.Errorf("排序值必须是整数: %q", raw)
			return row
		}
		row.Create.SortOrder, row.Update.SortOrder = &sortOrder, &sortOrder
	}
	return row
}

// parseCSVShelfFlag reads the 是否上架 column: the canonical true/false plus the
// spellings a Chinese spreadsheet is likely to carry.
func parseCSVShelfFlag(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "y", "是", "上架", "启用", "已上架":
		return true, nil
	case "false", "0", "no", "n", "否", "下架", "停用", "已下架":
		return false, nil
	}
	return false, fmt.Errorf("无法识别的上架状态: %q (可用 true/false/1/0/是/否/上架/下架)", raw)
}

// ToneImportRowError describes one rejected row.
type ToneImportRowError struct {
	Row     int    `json:"row"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// ToneImportResult is the report of one import request.
type ToneImportResult struct {
	Total    int                  `json:"total"`
	Created  int                  `json:"created"`
	Updated  int                  `json:"updated"`
	Failed   int                  `json:"failed"`
	Errors   []ToneImportRowError `json:"errors"`
	Warnings []string             `json:"warnings,omitempty"`
}

// fail records one rejected row, keeping the response bounded.
func (r *ToneImportResult) fail(row int, name string, message string) {
	r.Failed++
	if len(r.Errors) >= maxReportedRowErrors {
		return
	}
	r.Errors = append(r.Errors, ToneImportRowError{Row: row, Name: name, Message: message})
}

// ImportTones applies parsed rows one by one, so a single bad line never
// discards the rest of the file. The mode is validated before anything is
// written.
func ImportTones(rows []ToneImportRow, mode string, warnings []string) (ToneImportResult, error) {
	if mode != ImportModeUpsert && mode != ImportModeCreate {
		return ToneImportResult{}, fmt.Errorf(
			"非法导入模式 %q (可选 %s / %s)", mode, ImportModeCreate, ImportModeUpsert)
	}

	result := ToneImportResult{
		Total:    len(rows),
		Errors:   make([]ToneImportRowError, 0),
		Warnings: warnings,
	}
	for _, row := range rows {
		if row.Err != nil {
			result.fail(row.Row, row.Create.Name, row.Err.Error())
			continue
		}

		existingID, err := toneIDByName(row.Create.Name)
		if err != nil {
			return result, err
		}
		if existingID == 0 {
			if _, err := ToneInsert(&row.Create); err != nil {
				result.fail(row.Row, row.Create.Name, err.Error())
				continue
			}
			result.Created++
			continue
		}
		if mode == ImportModeCreate {
			result.fail(row.Row, row.Create.Name, "文风名称已存在（当前为仅新增模式）")
			continue
		}
		if _, err := ToneUpdate(existingID, &row.Update); err != nil {
			result.fail(row.Row, row.Create.Name, err.Error())
			continue
		}
		result.Updated++
	}

	if result.Failed > len(result.Errors) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("仅显示前 %d 条错误，共 %d 行未导入", len(result.Errors), result.Failed))
	}
	return result, nil
}

// toneIDByName returns the id stored under name, or 0 when the name is free.
func toneIDByName(name string) (uint, error) {
	t := &Tone{}
	err := db().Select("id").First(t, "name = ?", strings.TrimSpace(name)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lookup tone by name: %w", err)
	}
	return t.ID, nil
}
