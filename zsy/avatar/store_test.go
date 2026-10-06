package avatar_test

import (
	"testing"

	"github.com/QuantumNous/new-api/zsy/avatar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchNames answers one list query and returns the persona names in order, so
// the filter assertions state exactly what an operator would see.
func searchNames(t *testing.T, query avatar.AvatarListQuery) []string {
	t.Helper()
	result, err := avatar.AvatarSearch(query, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		names = append(names, item.Name)
	}
	return names
}

// ---------------------------------------------------------------------------
// Create / update / delete contract
// ---------------------------------------------------------------------------

func TestAvatarInsert_AppliesDocumentedDefaults(t *testing.T) {
	newAvatarTestDB(t)

	view, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{Name: "小雨"})
	require.NoError(t, err)

	assert.NotZero(t, view.ID)
	assert.Equal(t, "小雨", view.Name)
	assert.True(t, view.Enabled, "a new persona is published to the plaza")
	assert.Equal(t, 0, view.SortOrder)
	assert.Empty(t, view.Scenes)
	assert.NotNil(t, view.Scenes, "scenes is always an array, never null")
}

func TestAvatarInsert_KeepsTheStoredRowShape(t *testing.T) {
	newAvatarTestDB(t)
	sortOrder := 5
	enabled := false

	view, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{
		Name:          "  客服小雨  ",
		Description:   "  温柔的客服形象  ",
		ImageURL:      "  /uploads/images/202601/x.png  ",
		FullBodyURL:   "  /uploads/images/202601/x-full.png  ",
		FourViewURL:   "  /uploads/images/202601/x-four-view.png  ",
		ExpressionURL: "  /uploads/images/202601/x-expression.png  ",
		Gender:        "Female",
		AgeRange:      "YOUNG",
		Race:          "Asian",
		Scenes:        []string{"客服播报", "客服播报", "有声书", "  "},
		VoiceID:       "  zh_female_vv_uranus_bigtts  ",
		SortOrder:     &sortOrder,
		Enabled:       &enabled,
	})
	require.NoError(t, err)

	assert.Equal(t, "客服小雨", view.Name, "the name is trimmed")
	assert.Equal(t, "温柔的客服形象", view.Description)
	assert.Equal(t, "/uploads/images/202601/x.png", view.ImageURL)
	assert.Equal(t, "/uploads/images/202601/x-full.png", view.FullBodyURL)
	assert.Equal(t, "/uploads/images/202601/x-four-view.png", view.FourViewURL)
	assert.Equal(t, "/uploads/images/202601/x-expression.png", view.ExpressionURL)
	assert.Equal(t, "female", view.Gender, "the controlled vocabulary is lower-cased")
	assert.Equal(t, "young", view.AgeRange)
	assert.Equal(t, "asian", view.Race)
	assert.Equal(t, []string{"客服播报", "有声书"}, view.Scenes, "scenes are de-duplicated in order")
	assert.Equal(t, "zh_female_vv_uranus_bigtts", view.VoiceID, "a voice id keeps its case")
	assert.Equal(t, 5, view.SortOrder)
	assert.False(t, view.Enabled)
}

func TestAvatarInsert_RejectsADuplicateName(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "小雨", 0, true)

	_, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{Name: "小雨"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestAvatarInsert_ValidatesTheFieldContract(t *testing.T) {
	newAvatarTestDB(t)

	for _, tc := range []struct {
		name    string
		dto     avatar.AvatarCreateDTO
		wantMsg string
	}{
		{"missing name", avatar.AvatarCreateDTO{}, "形象名称不能为空"},
		{"unknown gender", avatar.AvatarCreateDTO{Name: "a", Gender: "robot"}, "非法性别"},
		{"unknown age range", avatar.AvatarCreateDTO{Name: "a", AgeRange: "ancient"}, "非法年龄段"},
		{"unknown race", avatar.AvatarCreateDTO{Name: "a", Race: "martian"}, "非法种族"},
		{"relative image path", avatar.AvatarCreateDTO{Name: "a", ImageURL: "images/x.png"}, "封面图地址必须是"},
		{"javascript image url", avatar.AvatarCreateDTO{Name: "a", ImageURL: "javascript:alert(1)"}, "封面图地址必须是"},
		{"protocol-relative image url", avatar.AvatarCreateDTO{Name: "a", ImageURL: "//cdn.example.com/x.png"}, "封面图地址必须是"},
		{"relative full-body path", avatar.AvatarCreateDTO{Name: "a", FullBodyURL: "images/full.png"}, "全身照地址必须是"},
		{"javascript four-view url", avatar.AvatarCreateDTO{Name: "a", FourViewURL: "javascript:alert(1)"}, "四视图地址必须是"},
		{"data expression url", avatar.AvatarCreateDTO{Name: "a", ExpressionURL: "data:image/png;base64,x"}, "表情图地址必须是"},
		{"too many scenes", avatar.AvatarCreateDTO{
			Name:   "a",
			Scenes: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
		}, "适合场景最多 8 个"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := avatar.AvatarInsert(&tc.dto)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestAvatarUpdate_OnlyTouchesProvidedFields(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	newVoiceID := "zh_male_narrator"
	updated, err := avatar.AvatarUpdate(created.ID, &avatar.AvatarUpdateDTO{VoiceID: &newVoiceID})
	require.NoError(t, err)

	assert.Equal(t, "zh_male_narrator", updated.VoiceID)
	assert.Equal(t, "客服小雨", updated.Name, "an omitted field keeps its stored value")
	assert.Equal(t, []string{"客服播报", "有声书"}, updated.Scenes)
	assert.Equal(t, "asian", updated.Race)
}

func TestAvatarUpdate_ClearsAFieldOnExplicitEmptyString(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	empty := ""
	updated, err := avatar.AvatarUpdate(created.ID, &avatar.AvatarUpdateDTO{VoiceID: &empty})
	require.NoError(t, err)

	assert.Empty(t, updated.VoiceID, "an explicit empty string detaches the voice")
	assert.Empty(t, updated.VoiceSampleURL)
}

func TestAvatarUpdate_ReplacesAndClearsOnePicture(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	replaced := "https://cdn.example.com/persona/xiaoyu-expression-v2.png"
	updated, err := avatar.AvatarUpdate(created.ID, &avatar.AvatarUpdateDTO{ExpressionURL: &replaced})
	require.NoError(t, err)

	assert.Equal(t, replaced, updated.ExpressionURL)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-full.png", updated.FullBodyURL,
		"an omitted picture keeps its stored value")

	empty := ""
	cleared, err := avatar.AvatarUpdate(created.ID, &avatar.AvatarUpdateDTO{FullBodyURL: &empty})
	require.NoError(t, err)

	assert.Empty(t, cleared.FullBodyURL, "an explicit empty string removes that one picture")
	assert.Equal(t, replaced, cleared.ExpressionURL, "the other pictures stay untouched")
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu.png", cleared.ImageURL)
}

func TestAvatarUpdate_ReportsAMissingRow(t *testing.T) {
	newAvatarTestDB(t)

	_, err := avatar.AvatarUpdate(42, &avatar.AvatarUpdateDTO{})
	assert.ErrorIs(t, err, avatar.ErrAvatarNotFound)
}

func TestAvatarUpdate_RejectsADuplicateName(t *testing.T) {
	newAvatarTestDB(t)
	first := insertAvatar(t, "小雨", 0, true)
	insertAvatar(t, "小雪", 1, true)

	duplicate := "小雪"
	_, err := avatar.AvatarUpdate(first.ID, &avatar.AvatarUpdateDTO{Name: &duplicate})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestAvatarDelete_FreesTheNameForReuse(t *testing.T) {
	newAvatarTestDB(t)
	created := insertAvatar(t, "小雨", 0, true)

	name, err := avatar.AvatarDelete(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "小雨", name)

	_, err = avatar.AvatarGetByID(created.ID, nil)
	assert.ErrorIs(t, err, avatar.ErrAvatarNotFound)

	recreated, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{Name: "小雨"})
	require.NoError(t, err, "a hard delete frees the unique name immediately")
	assert.Equal(t, "小雨", recreated.Name)
	assert.True(t, recreated.Enabled)
}

// ---------------------------------------------------------------------------
// List contract
// ---------------------------------------------------------------------------

func TestAvatarSearch_OrdersBySortOrderThenID(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "second", 1, true)
	insertAvatar(t, "first", 0, true)
	insertAvatar(t, "third", 0, true)

	names := searchNames(t, avatar.AvatarListQuery{})
	assert.Equal(t, []string{"first", "third", "second"}, names)
}

func TestAvatarSearch_ReportsThePagingEnvelope(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "second", 1, true)
	insertAvatar(t, "first", 0, true)

	result, err := avatar.AvatarSearch(avatar.AvatarListQuery{}, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, result.Total)
	assert.Equal(t, 1, result.TotalPages)

	// A page past the end is not an error: it answers empty and echoes the page.
	past, err := avatar.AvatarSearch(avatar.AvatarListQuery{Page: 5}, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, past.Page)
	assert.Empty(t, past.Items)
	assert.EqualValues(t, 2, past.Total)
}

func TestAvatarSearch_ClampsPagination(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "only", 0, true)

	for _, tc := range []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{"page below one", 0, 0, 1, 20},
		{"negative page size", 2, -5, 2, 20},
		{"page size above the cap", 1, 500, 1, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := avatar.AvatarSearch(
				avatar.AvatarListQuery{Page: tc.page, PageSize: tc.pageSize}, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.wantPage, result.Page)
			assert.Equal(t, tc.wantPageSize, result.PageSize)
		})
	}
}

func TestAvatarSearch_AnswersTheAdminAndPublicFilters(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t) // female / young / asian / zh_female_vv_uranus_bigtts
	insertAvatar(t, "下架形象", 2, false)

	for _, tc := range []struct {
		name  string
		query avatar.AvatarListQuery
		want  []string
	}{
		{"gender", avatar.AvatarListQuery{Gender: "FEMALE"}, []string{"客服小雨"}},
		{"gender miss", avatar.AvatarListQuery{Gender: "male"}, []string{}},
		{"age range", avatar.AvatarListQuery{AgeRange: "young"}, []string{"客服小雨"}},
		{"race", avatar.AvatarListQuery{Race: "asian"}, []string{"客服小雨"}},
		{"voice id", avatar.AvatarListQuery{VoiceID: "zh_female_vv_uranus_bigtts"}, []string{"客服小雨"}},
		{"voice id miss", avatar.AvatarListQuery{VoiceID: "zh_male_missing"}, []string{}},
		{"keyword on the description", avatar.AvatarListQuery{Keyword: "客服"}, []string{"客服小雨"}},
		{"keyword on the voice id", avatar.AvatarListQuery{Keyword: "uranus"}, []string{"客服小雨"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, searchNames(t, tc.query))
		})
	}

	t.Run("enabled false", func(t *testing.T) {
		enabled := false
		result, err := avatar.AvatarSearch(avatar.AvatarListQuery{Enabled: &enabled}, nil)
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		assert.Equal(t, "下架形象", result.Items[0].Name)
	})
}

// ---------------------------------------------------------------------------
// Voice-catalog link
// ---------------------------------------------------------------------------

func TestResolveVoiceSamples_ReadsTheRealVoiceTable(t *testing.T) {
	db := newAvatarTestDB(t)
	insertCatalogVoice(t, db, "Vivi 2.0", "zh_female_vv_uranus_bigtts",
		"/uploads/voices/202601/vivi.wav", "vivi.wav")
	created := insertRichAvatar(t)

	got, err := avatar.TestHookCatalogVoiceSamples([]string{
		"zh_female_vv_uranus_bigtts", "zh_female_vv_uranus_bigtts", "zh_male_missing",
	})
	require.NoError(t, err)
	require.Contains(t, got, "zh_female_vv_uranus_bigtts")
	assert.Equal(t, "Vivi 2.0", got["zh_female_vv_uranus_bigtts"].Name)
	assert.Equal(t, "/uploads/voices/202601/vivi.wav", got["zh_female_vv_uranus_bigtts"].AudioURL)
	assert.Equal(t, "vivi.wav", got["zh_female_vv_uranus_bigtts"].AudioName)
	assert.NotContains(t, got, "zh_male_missing", "an uncatalogued voice is simply absent")

	view, err := avatar.AvatarGetByID(created.ID, nil)
	require.NoError(t, err)
	assert.True(t, view.VoiceAvailable)
	assert.Equal(t, "Vivi 2.0", view.VoiceName)
	assert.Equal(t, "/uploads/voices/202601/vivi.wav", view.VoiceSampleURL)
	assert.Equal(t, "vivi.wav", view.VoiceSampleName)
}

func TestAvatarSearch_ReportsAnUncataloguedVoice(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)

	result, err := avatar.AvatarSearch(avatar.AvatarListQuery{}, nil)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)

	item := result.Items[0]
	assert.False(t, item.VoiceAvailable, "the persona names a voice that is not in the catalog")
	assert.Equal(t, "zh_female_vv_uranus_bigtts", item.VoiceID, "the id is still reported verbatim")
	assert.Empty(t, item.VoiceSampleURL)
}

func TestAvatarSearch_FollowsTheVoiceSampleWhenItChanges(t *testing.T) {
	db := newAvatarTestDB(t)
	insertCatalogVoice(t, db, "Vivi", "zh_female_vv_uranus_bigtts", "/uploads/voices/old.wav", "old.wav")
	created := insertRichAvatar(t)

	before, err := avatar.AvatarGetByID(created.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "/uploads/voices/old.wav", before.VoiceSampleURL)

	// The voice catalog re-uploads the sample; the persona points at the voice,
	// not at the file, so the new sample shows up without touching the row.
	require.NoError(t, db.Exec(
		`UPDATE zsy_voices SET audio_url = ?, audio_name = ? WHERE voice_type = ?`,
		"/uploads/voices/new.wav", "new.wav", "zh_female_vv_uranus_bigtts").Error)

	after, err := avatar.AvatarGetByID(created.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, "/uploads/voices/new.wav", after.VoiceSampleURL)
	assert.Equal(t, "new.wav", after.VoiceSampleName)
}
