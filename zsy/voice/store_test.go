package voice_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/voice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Store layer: validation, pagination contract, filters, update/delete
// semantics. These are the rules the admin UI and third-party callers both
// depend on.
// ---------------------------------------------------------------------------

func TestVoiceInsert_RejectsInvalidFields(t *testing.T) {
	newVoiceTestDB(t)

	// Every accepted case inserts a row, so each case gets its own name: the
	// unique index on name would otherwise decide the outcome instead of the
	// rule under test.
	valid := func(name string) *voice.VoiceCreateDTO {
		return &voice.VoiceCreateDTO{
			Name:        name,
			VoiceType:   "zh_female_xiaomei",
			Description: "温柔女声",
			AudioURL:    "/uploads/voices/202601/sample.mp3",
		}
	}

	cases := []struct {
		name    string
		mutate  func(dto *voice.VoiceCreateDTO)
		wantErr string
	}{
		{
			name:    "empty name",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Name = "   " },
			wantErr: "音色名称不能为空",
		},
		{
			name:    "name longer than the indexed column",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Name = strings.Repeat("音", 192) },
			wantErr: "音色名称过长",
		},
		{
			name:    "empty voice_type",
			mutate:  func(d *voice.VoiceCreateDTO) { d.VoiceType = "" },
			wantErr: "voice_type 不能为空",
		},
		{
			name:    "voice_type longer than the indexed column",
			mutate:  func(d *voice.VoiceCreateDTO) { d.VoiceType = strings.Repeat("v", 192) },
			wantErr: "voice_type 过长",
		},
		{
			name:    "description longer than the cap",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Description = strings.Repeat("a", 2001) },
			wantErr: "简介过长",
		},
		{
			name:    "negative audio size",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioSize = -1 },
			wantErr: "音频文件大小不能为负数",
		},
		{
			name:    "sort order out of range",
			mutate:  func(d *voice.VoiceCreateDTO) { d.SortOrder = intPtr(1_000_001) },
			wantErr: "排序值超出范围",
		},
		{
			name:    "javascript url",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "javascript:alert(1)" },
			wantErr: "音频地址必须是",
		},
		{
			name:    "protocol relative url",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "//evil.example/a.mp3" },
			wantErr: "音频地址必须是",
		},
		{
			name:    "bare host is not a url",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "evil.example/a.mp3" },
			wantErr: "音频地址必须是",
		},
		{
			name:    "audio url longer than the column",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "https://example.com/" + strings.Repeat("a", 800) },
			wantErr: "音频地址过长",
		},
		{
			name:    "empty audio url is allowed",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "" },
			wantErr: "",
		},
		{
			name:    "https audio url is allowed",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AudioURL = "https://cdn.example.com/a.mp3" },
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := valid("校验音色-" + tc.name)
			tc.mutate(dto)
			_, err := voice.VoiceInsert(dto)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestVoiceInsert_AppliesDocumentedDefaults(t *testing.T) {
	newVoiceTestDB(t)

	view, err := voice.VoiceInsert(&voice.VoiceCreateDTO{Name: "默认音色", VoiceType: "zh_male_default"})
	require.NoError(t, err)

	assert.True(t, view.Enabled, "a voice created without an explicit flag is on shelf")
	assert.Zero(t, view.SortOrder)
	assert.NotZero(t, view.CreatedAt, "createdAt is filled by the store")
	assert.GreaterOrEqual(t, view.UpdatedAt, view.CreatedAt)
}

func TestVoiceInsert_TrimsWhitespace(t *testing.T) {
	newVoiceTestDB(t)

	view, err := voice.VoiceInsert(&voice.VoiceCreateDTO{
		Name:      "  带空格的音色  ",
		VoiceType: "  voice_type_x  ",
	})
	require.NoError(t, err)
	assert.Equal(t, "带空格的音色", view.Name)
	assert.Equal(t, "voice_type_x", view.VoiceType)
}

func TestVoiceInsert_RejectsDuplicateName(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "重复音色", "vt-1", 0, true)

	_, err := voice.VoiceInsert(&voice.VoiceCreateDTO{Name: "重复音色", VoiceType: "vt-2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestVoiceSearch_NormalizesPagination(t *testing.T) {
	newVoiceTestDB(t)
	for i := 0; i < 5; i++ {
		insertVoice(t, nameFor(i), "vt", i, true)
	}

	page1, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.EqualValues(t, 5, page1.Total)
	assert.Equal(t, 1, page1.Page)
	assert.Equal(t, 2, page1.PageSize)
	assert.Equal(t, 3, page1.TotalPages)
	require.Len(t, page1.Items, 2)

	lastPage, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 3, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, lastPage.Items, 1)
	assert.Equal(t, 3, lastPage.Page)

	outOfRange, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 99, PageSize: 2})
	require.NoError(t, err)
	assert.Empty(t, outOfRange.Items)
	assert.EqualValues(t, 5, outOfRange.Total)
	assert.Equal(t, 99, outOfRange.Page, "an out-of-range page is echoed, not clamped")

	invalid, err := voice.VoiceSearch(voice.VoiceListQuery{Page: -3, PageSize: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, invalid.Page)
	assert.Equal(t, 20, invalid.PageSize)
	assert.Len(t, invalid.Items, 5)

	capped, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 1, PageSize: 5000})
	require.NoError(t, err)
	assert.Equal(t, 100, capped.PageSize, "page size is capped to protect the gateway")
}

func TestVoiceSearch_OrdersBySortOrderThenID(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "第三", "vt", 10, true)
	insertVoice(t, "第一", "vt", -5, true)
	second := insertVoice(t, "第二", "vt", 0, true)
	tied := insertVoice(t, "并列", "vt", 0, true)

	result, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result.Items, 4)
	assert.Equal(t, []string{"第一", "第二", "并列", "第三"}, voiceNames(result.Items))
	assert.Less(t, second.ID, tied.ID, "the fixture must tie in sort order to prove the id tie-break")
}

func TestVoiceSearch_FiltersByEnabledKeywordAndVoiceType(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "小美", "zh_female_xiaomei", 0, true)
	insertVoice(t, "小北", "zh_male_xiaobei", 0, true)
	insertVoice(t, "下架音色", "zh_female_offline", 0, false)

	enabled := true
	onShelf, err := voice.VoiceSearch(voice.VoiceListQuery{Enabled: &enabled, Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"小美", "小北"}, voiceNames(onShelf.Items))

	disabled := false
	offShelf, err := voice.VoiceSearch(voice.VoiceListQuery{Enabled: &disabled, Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"下架音色"}, voiceNames(offShelf.Items))

	byKeyword, err := voice.VoiceSearch(voice.VoiceListQuery{Keyword: "xiaobei", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"小北"}, voiceNames(byKeyword.Items))

	byVoiceType, err := voice.VoiceSearch(voice.VoiceListQuery{VoiceType: "zh_female_xiaomei", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"小美"}, voiceNames(byVoiceType.Items))

	noMatch, err := voice.VoiceSearch(voice.VoiceListQuery{Keyword: "不存在", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, noMatch.Items)
	assert.Zero(t, noMatch.Total)
	assert.Zero(t, noMatch.TotalPages)
}

func TestVoiceUpdate_AppliesOnlyProvidedFields(t *testing.T) {
	newVoiceTestDB(t)
	created := insertVoice(t, "原始音色", "vt-original", 3, true)

	description := "更新后的简介"
	updated, err := voice.VoiceUpdate(created.ID, &voice.VoiceUpdateDTO{Description: &description})
	require.NoError(t, err)
	assert.Equal(t, "更新后的简介", updated.Description)
	assert.Equal(t, "原始音色", updated.Name, "omitted fields keep their stored value")
	assert.Equal(t, "vt-original", updated.VoiceType)
	assert.Equal(t, 3, updated.SortOrder)
	assert.True(t, updated.Enabled)

	offShelf := false
	updated, err = voice.VoiceUpdate(created.ID, &voice.VoiceUpdateDTO{Enabled: &offShelf})
	require.NoError(t, err)
	assert.False(t, updated.Enabled)

	// An explicit empty string is the documented way to clear a field.
	empty := ""
	updated, err = voice.VoiceUpdate(created.ID, &voice.VoiceUpdateDTO{AudioURL: &empty, AudioSize: int64Ptr(0)})
	require.NoError(t, err)
	assert.Empty(t, updated.AudioURL)
	assert.Zero(t, updated.AudioSize)
}

func TestVoiceUpdate_RejectsDuplicateNameAndMissingRow(t *testing.T) {
	newVoiceTestDB(t)
	first := insertVoice(t, "音色甲", "vt-a", 0, true)
	insertVoice(t, "音色乙", "vt-b", 0, true)

	rename := "音色乙"
	_, err := voice.VoiceUpdate(first.ID, &voice.VoiceUpdateDTO{Name: &rename})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")

	badName := "   "
	_, err = voice.VoiceUpdate(first.ID, &voice.VoiceUpdateDTO{Name: &badName})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "音色名称不能为空")

	_, err = voice.VoiceUpdate(9999, &voice.VoiceUpdateDTO{Description: stringPtr("x")})
	require.ErrorIs(t, err, voice.ErrVoiceNotFound)
}

func TestVoiceDelete_FreesTheName(t *testing.T) {
	newVoiceTestDB(t)
	created := insertVoice(t, "待删除音色", "vt-del", 0, true)

	name, err := voice.VoiceDelete(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "待删除音色", name)

	_, err = voice.VoiceGetByID(created.ID)
	require.ErrorIs(t, err, voice.ErrVoiceNotFound)

	// The catalog must not keep a hidden row occupying the unique name.
	recreated, err := voice.VoiceInsert(&voice.VoiceCreateDTO{Name: "待删除音色", VoiceType: "vt-del-2"})
	require.NoError(t, err, "the deleted name is reusable")
	assert.Equal(t, "待删除音色", recreated.Name)

	_, err = voice.VoiceGetByID(recreated.ID)
	require.NoError(t, err)

	_, err = voice.VoiceDelete(999_999)
	require.ErrorIs(t, err, voice.ErrVoiceNotFound)
}

func nameFor(i int) string {
	return string(rune('A' + i))
}

// ---------------------------------------------------------------------------
// Attribute fields: gender, age range and suitable scenes
// ---------------------------------------------------------------------------

func TestVoiceInsert_NormalizesAttributes(t *testing.T) {
	newVoiceTestDB(t)

	view, err := voice.VoiceInsert(&voice.VoiceCreateDTO{
		Name:      "属性音色",
		VoiceType: "vt-attr",
		// A spreadsheet writes "Female"; the API must store the vocabulary value.
		Gender:   "  Female ",
		AgeRange: "SENIOR",
		Language: " PT-BR ",
		// Duplicates and blank entries in a pasted list are dropped.
		Scenes: []string{" 客服播报 ", "", "有声书", "客服播报"},
	})
	require.NoError(t, err)

	assert.Equal(t, "female", view.Gender)
	assert.Equal(t, "senior", view.AgeRange)
	assert.Equal(t, "pt-br", view.Language)
	assert.Equal(t, []string{"客服播报", "有声书"}, view.Scenes)

	// The stored form is the canonical comma-separated string, which is what the
	// CSV export writes and the import parses back.
	stored, err := voice.VoiceGetByID(view.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"客服播报", "有声书"}, stored.Scenes)
}

func TestVoiceInsert_RejectsInvalidAttributes(t *testing.T) {
	newVoiceTestDB(t)

	cases := []struct {
		name     string
		mutate   func(dto *voice.VoiceCreateDTO)
		wantErr  string
		accepted bool
	}{
		{
			name:    "unknown gender",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Gender = "unknown" },
			wantErr: "非法性别",
		},
		{
			name:    "unknown age range",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AgeRange = "40s" },
			wantErr: "非法年龄段",
		},
		{
			name:    "language with upper case is rejected after normalization",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Language = "zh-CN!" },
			wantErr: "非法语言代码",
		},
		{
			name:    "language longer than the column",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Language = strings.Repeat("a", 17) },
			wantErr: "语言代码过长",
		},
		{
			name:     "provider specific language codes are accepted",
			mutate:   func(d *voice.VoiceCreateDTO) { d.Language = "mx" },
			accepted: true,
		},
		{
			name:    "avatar url must be loadable",
			mutate:  func(d *voice.VoiceCreateDTO) { d.AvatarURL = "javascript:alert(1)" },
			wantErr: "头像地址必须是",
		},
		{
			name:     "absolute avatar url is accepted",
			mutate:   func(d *voice.VoiceCreateDTO) { d.AvatarURL = "https://cdn.example.com/a.png" },
			accepted: true,
		},
		{
			name:     "empty vocabulary is allowed",
			mutate:   func(d *voice.VoiceCreateDTO) { d.Gender = ""; d.AgeRange = "" },
			accepted: true,
		},
		{
			name:    "too many scenes",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Scenes = manyScenes(9) },
			wantErr: "适合场景最多 8 个",
		},
		{
			name:     "the scene limit itself is accepted",
			mutate:   func(d *voice.VoiceCreateDTO) { d.Scenes = manyScenes(8) },
			accepted: true,
		},
		{
			name:    "scene longer than the cap",
			mutate:  func(d *voice.VoiceCreateDTO) { d.Scenes = []string{strings.Repeat("场", 25)} },
			wantErr: "单个场景过长",
		},
		{
			name:     "scene at the cap is accepted",
			mutate:   func(d *voice.VoiceCreateDTO) { d.Scenes = []string{strings.Repeat("场", 24)} },
			accepted: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := &voice.VoiceCreateDTO{
				Name:      "属性校验-" + tc.name,
				VoiceType: "vt-attr",
			}
			tc.mutate(dto)
			_, err := voice.VoiceInsert(dto)
			if tc.accepted {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestVoiceUpdate_AppliesAttributePointersOnly(t *testing.T) {
	newVoiceTestDB(t)
	created := insertRichVoice(t)

	gender := "neutral"
	updated, err := voice.VoiceUpdate(created.ID, &voice.VoiceUpdateDTO{Gender: &gender})
	require.NoError(t, err)
	assert.Equal(t, "neutral", updated.Gender)
	assert.Equal(t, "young", updated.AgeRange, "an omitted attribute keeps its stored value")
	assert.Equal(t, []string{"客服播报", "有声书"}, updated.Scenes)

	empty := voice.ScenesInput{}
	cleared, err := voice.VoiceUpdate(created.ID, &voice.VoiceUpdateDTO{Scenes: &empty})
	require.NoError(t, err)
	assert.Empty(t, cleared.Scenes)
	assert.Equal(t, "young", cleared.AgeRange)
}

func TestVoiceSearch_FiltersByAttributes(t *testing.T) {
	newVoiceTestDB(t)
	insertRichVoice(t)
	insertVoice(t, "小北", "zh_male_xiaobei", 1, true)

	byGender, err := voice.VoiceSearch(voice.VoiceListQuery{Gender: "Female", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"客服女声"}, voiceNames(byGender.Items), "the gender filter matches case-insensitively")

	byAge, err := voice.VoiceSearch(voice.VoiceListQuery{AgeRange: "young", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"客服女声"}, voiceNames(byAge.Items))

	// The keyword search reaches the new columns too, so an operator can type a
	// scene name into the same box.
	byScene, err := voice.VoiceSearch(voice.VoiceListQuery{Keyword: "有声书", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"客服女声"}, voiceNames(byScene.Items))

	byGenderWord, err := voice.VoiceSearch(voice.VoiceListQuery{Keyword: "female", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"客服女声"}, voiceNames(byGenderWord.Items))

	byLanguage, err := voice.VoiceSearch(voice.VoiceListQuery{Language: "ZH", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"客服女声"}, voiceNames(byLanguage.Items), "the language filter matches case-insensitively")

	noMatch, err := voice.VoiceSearch(voice.VoiceListQuery{Gender: "male", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, noMatch.Items)
}

// manyScenes builds n distinct scenes.
func manyScenes(n int) []string {
	scenes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		scenes = append(scenes, "场景"+nameFor(i))
	}
	return scenes
}

func voiceNames(items []*voice.VoiceView) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func intPtr(v int) *int          { return &v }
func int64Ptr(v int64) *int64    { return &v }
func stringPtr(v string) *string { return &v }
