package voice

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// The Voice Plaza CSV contract -------------------------------------------------
//
// Export writes a canonical English header; import accepts that header plus the
// Chinese spellings an operator is likely to type, so a spreadsheet filled in by
// hand imports without renaming columns. Everything about the round trip lives
// here: the same code writes an export and feeds an import.

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
	// ImportModeUpsert creates unknown names and updates the voices already
	// stored under a name from the file.
	ImportModeUpsert = "upsert"
	// ImportModeCreate only creates; an existing name is reported as a failed
	// row so a merge never happens by accident.
	ImportModeCreate = "create"
)

// csvColumn is one exported column: the canonical header written by the export
// plus the header spellings accepted on import.
type csvColumn struct {
	key     string
	aliases []string
}

// csvColumns fixes the export column order and the accepted import headers. The
// required pair (name, voice_type) is validated by ParseVoicesCSV.
var csvColumns = []csvColumn{
	{key: "name", aliases: []string{"音色名称", "名称", "音色名"}},
	{key: "description", aliases: []string{"简介", "介绍", "描述"}},
	{key: "voice_type", aliases: []string{"音色标识", "声音类型", "音色类型"}},
	{key: "gender", aliases: []string{"性别"}},
	{key: "age_range", aliases: []string{"年龄段", "年龄"}},
	{key: "language", aliases: []string{"语言", "语种", "语言代码"}},
	{key: "scenes", aliases: []string{"scene", "适合场景", "场景", "适用场景"}},
	{key: "avatar_url", aliases: []string{"头像", "头像地址", "头像URL", "封面", "图标"}},
	{key: "audio_url", aliases: []string{"音频地址", "示例音频", "音频", "音频示例URL"}},
	{key: "audio_name", aliases: []string{"音频文件名"}},
	{key: "audio_size", aliases: []string{"音频大小"}},
	{key: "enabled", aliases: []string{"是否上架", "上架", "启用"}},
	{key: "sort_order", aliases: []string{"排序", "排序值", "顺序"}},
}

// requiredCSVColumns are the columns an import cannot work without, with the
// label used in the error message.
var requiredCSVColumns = []struct {
	key   string
	label string
}{
	{key: "name", label: "name（音色名称）"},
	{key: "voice_type", label: "voice_type"},
}

// csvColumnIndex maps every accepted header spelling (normalized) to the
// canonical column key.
var csvColumnIndex = buildCSVColumnIndex()

func buildCSVColumnIndex() map[string]string {
	index := make(map[string]string, len(csvColumns)*3)
	for _, column := range csvColumns {
		index[normalizeCSVHeader(column.key)] = column.key
		for _, alias := range column.aliases {
			index[normalizeCSVHeader(alias)] = column.key
		}
	}
	return index
}

// normalizeCSVHeader lower-cases a header cell and drops the separators an
// operator may type ("Voice Type", "age-range", a UTF-8 BOM left by Excel), so
// "Voice_Type" and "voice type" both reach the same column.
func normalizeCSVHeader(raw string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(trimmed))
}

// WriteVoicesCSV writes the canonical header and one row per voice, prefixed
// with a UTF-8 BOM: without it Excel on Windows reads the file as the local
// code page and mangles every Chinese value.
func WriteVoicesCSV(w io.Writer, rows []Voice) error {
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
		if err := writer.Write(voiceCSVRecord(&rows[i])); err != nil {
			return fmt.Errorf("写入 CSV 第 %d 行失败: %w", i+2, err)
		}
	}
	writer.Flush()
	return writer.Error()
}

// voiceCSVRecord renders one stored voice in csvColumns order. Scenes go out as
// the canonical comma-separated string, which the CSV writer quotes, so an
// export re-imports unchanged.
func voiceCSVRecord(v *Voice) []string {
	return []string{
		v.Name,
		v.Description,
		v.VoiceType,
		v.Gender,
		v.AgeRange,
		v.Language,
		v.Scenes,
		v.AvatarURL,
		v.AudioURL,
		v.AudioName,
		strconv.FormatInt(v.AudioSize, 10),
		strconv.FormatBool(v.Enabled),
		strconv.Itoa(v.SortOrder),
	}
}

// VoiceImportRow is one parsed record: Create is the shape used when the name is
// free, Update the shape applied when it already exists. Update only carries the
// columns the file actually had, so an upsert never blanks a field the
// spreadsheet omitted.
type VoiceImportRow struct {
	// Row is the record number in the uploaded file, counting the header as
	// row 1, so an error names the line the operator must fix.
	Row    int
	Create VoiceCreateDTO
	Update VoiceUpdateDTO
	// Err carries a row-local parse failure (an unreadable number or flag).
	// ImportVoices reports it as a failed row instead of discarding the file.
	Err error
}

// ParseVoicesCSV reads an uploaded CSV into rows. Structural problems — an empty
// file, a missing required column, an unreadable record, too many rows — fail
// the whole request, because guessing a column mapping would import the wrong
// data. Everything else is left to per-row validation.
func ParseVoicesCSV(r io.Reader) ([]VoiceImportRow, []string, error) {
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

	warnings := make([]string, 0, 1)
	if len(unknown) > 0 {
		warnings = append(warnings, "已忽略无法识别的列: "+strings.Join(unknown, ", "))
	}

	rows := make([]VoiceImportRow, 0, 16)
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
		row := parseVoiceCSVRow(cells, index)
		row.Row = record
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, warnings, errors.New("CSV 中没有可导入的数据行")
	}
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

// parseVoiceCSVRow maps one record onto the create/update shapes. A cell that
// cannot be read (a non-numeric size, an unknown shelf flag) is recorded on the
// row instead of failing the file.
func parseVoiceCSVRow(cells []string, index map[string]int) VoiceImportRow {
	row := VoiceImportRow{}
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
	if raw, ok := value("voice_type"); ok {
		row.Create.VoiceType, row.Update.VoiceType = raw, &raw
	}
	if raw, ok := value("gender"); ok {
		row.Create.Gender, row.Update.Gender = raw, &raw
	}
	if raw, ok := value("age_range"); ok {
		row.Create.AgeRange, row.Update.AgeRange = raw, &raw
	}
	if raw, ok := value("language"); ok {
		row.Create.Language, row.Update.Language = raw, &raw
	}
	if raw, ok := value("scenes"); ok {
		scenes := ScenesInput(splitScenes(raw))
		row.Create.Scenes, row.Update.Scenes = scenes, &scenes
	}
	if raw, ok := value("avatar_url"); ok {
		row.Create.AvatarURL, row.Update.AvatarURL = raw, &raw
	}
	if raw, ok := value("audio_url"); ok {
		row.Create.AudioURL, row.Update.AudioURL = raw, &raw
	}
	if raw, ok := value("audio_name"); ok {
		row.Create.AudioName, row.Update.AudioName = raw, &raw
	}
	if raw, ok := value("audio_size"); ok && raw != "" {
		size, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			row.Err = fmt.Errorf("音频大小必须是整数: %q", raw)
			return row
		}
		row.Create.AudioSize, row.Update.AudioSize = size, &size
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

// VoiceImportRowError describes one rejected row.
type VoiceImportRowError struct {
	Row     int    `json:"row"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// VoiceImportResult is the report of one import request.
type VoiceImportResult struct {
	Total    int                   `json:"total"`
	Created  int                   `json:"created"`
	Updated  int                   `json:"updated"`
	Failed   int                   `json:"failed"`
	Errors   []VoiceImportRowError `json:"errors"`
	Warnings []string              `json:"warnings,omitempty"`
}

// fail records one rejected row, keeping the response bounded.
func (r *VoiceImportResult) fail(row int, name string, message string) {
	r.Failed++
	if len(r.Errors) >= maxReportedRowErrors {
		return
	}
	r.Errors = append(r.Errors, VoiceImportRowError{Row: row, Name: name, Message: message})
}

// ImportVoices applies parsed rows one by one, so a single bad line never
// discards the rest of the file. The mode is validated before anything is
// written.
func ImportVoices(rows []VoiceImportRow, mode string, warnings []string) (VoiceImportResult, error) {
	if mode != ImportModeUpsert && mode != ImportModeCreate {
		return VoiceImportResult{}, fmt.Errorf(
			"非法导入模式 %q (可选 %s / %s)", mode, ImportModeCreate, ImportModeUpsert)
	}

	result := VoiceImportResult{
		Total:    len(rows),
		Errors:   make([]VoiceImportRowError, 0),
		Warnings: warnings,
	}
	for _, row := range rows {
		if row.Err != nil {
			result.fail(row.Row, row.Create.Name, row.Err.Error())
			continue
		}

		existingID, err := voiceIDByName(row.Create.Name)
		if err != nil {
			return result, err
		}
		if existingID == 0 {
			if _, err := VoiceInsert(&row.Create); err != nil {
				result.fail(row.Row, row.Create.Name, err.Error())
				continue
			}
			result.Created++
			continue
		}
		if mode == ImportModeCreate {
			result.fail(row.Row, row.Create.Name, "音色名称已存在（当前为仅新增模式）")
			continue
		}
		if _, err := VoiceUpdate(existingID, &row.Update); err != nil {
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

// voiceIDByName returns the id stored under name, or 0 when the name is free.
func voiceIDByName(name string) (uint, error) {
	v := &Voice{}
	err := db().Select("id").First(v, "name = ?", strings.TrimSpace(name)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lookup voice by name: %w", err)
	}
	return v.ID, nil
}
