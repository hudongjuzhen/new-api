package runninghub_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Display order of the app list (App.Sort) — the drag-and-drop contract of
// /rh-apps.
//
// Both the admin list and the public app center order by `sort DESC, id DESC`:
// a larger value shows earlier, and the page persists a drag by posting the
// rearranged rows to /dashboard/zsy/rh/apps/reorder. The tests below pin the
// three properties the page depends on — a new app opens on top, a reorder is
// reproduced exactly by the next list call, and a reorder sent from a filtered
// view leaves the rows it cannot see alone — plus the rejection paths that keep
// a stale view from half-applying.
// ---------------------------------------------------------------------------

const reorderRoute = "/apps/reorder"

// mountSortRoutes wires the two admin handlers these tests exercise.
func mountSortRoutes(env *rhITestEnv) {
	env.router.GET("/apps", runninghub.TestHookListApps)
	env.router.POST(reorderRoute, runninghub.TestHookReorderApps)
}

// appRows lists the apps through the admin endpoint and returns the decoded
// items in the order it answered with.
func appRows(t *testing.T, env *rhITestEnv) []map[string]any {
	t.Helper()
	w, raw := doJSON(t, env.router, http.MethodGet, "/apps", nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", string(raw))
	envelope := parseAPIEnvelope(t, raw)
	require.True(t, envelope.Success, "body: %s", string(raw))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data missing; body=%s", string(raw))
	items, ok := data["items"].([]any)
	require.True(t, ok, "items missing; body=%s", string(raw))
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		require.True(t, ok, "item is not an object: %v", item)
		rows = append(rows, row)
	}
	return rows
}

// namesOf lists the app names in response order.
func namesOf(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, fmt.Sprint(row["name"]))
	}
	return out
}

// sortsOf lists the persisted sort values in response order.
func sortsOf(t *testing.T, rows []map[string]any) []int {
	t.Helper()
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		value, ok := row["sort"].(float64)
		require.True(t, ok, "sort missing or not a number: %v", row)
		out = append(out, int(value))
	}
	return out
}

// TestAppSort_NewAppOpensOnTopOfTheList pins the order of a list nobody has
// dragged yet: it must stay the `id DESC` order the page showed before App.Sort
// existed, and a newly created app must still open at the top.
func TestAppSort_NewAppOpensOnTopOfTheList(t *testing.T) {
	env := newRHITestEnv(t, "8701-sort-new")
	mountSortRoutes(env)

	env.createApp("App A", "8701-app-a", false, 0, 1.0)
	env.createApp("App B", "8701-app-b", false, 0, 1.0)
	env.createApp("App C", "8701-app-c", false, 0, 1.0)

	rows := appRows(t, env)
	assert.Equal(t, []string{"App C", "App B", "App A"}, namesOf(rows))
	assert.Equal(t, []int{3, 2, 1}, sortsOf(t, rows))

	env.createApp("App D", "8701-app-d", false, 0, 1.0)

	rows = appRows(t, env)
	assert.Equal(t, []string{"App D", "App C", "App B", "App A"}, namesOf(rows))
	assert.Equal(t, []int{4, 3, 2, 1}, sortsOf(t, rows))
}

// TestAppReorder_ReproducesTheSubmittedOrder covers the drag itself: the page
// posts the rows it rearranged, front first, and the next list call must answer
// with exactly that sequence.
func TestAppReorder_ReproducesTheSubmittedOrder(t *testing.T) {
	env := newRHITestEnv(t, "8702-sort-reorder")
	mountSortRoutes(env)

	appA := env.createApp("App A", "8702-app-a", false, 0, 1.0)
	appB := env.createApp("App B", "8702-app-b", false, 0, 1.0)
	appC := env.createApp("App C", "8702-app-c", false, 0, 1.0)
	require.Equal(t, []string{"App C", "App B", "App A"}, namesOf(appRows(t, env)))

	// The admin drags App A from the bottom to the front.
	changed, err := runninghub.AppReorder([]uint{appA, appC, appB})
	require.NoError(t, err)
	assert.Equal(t, 3, changed, "every position changed")

	rows := appRows(t, env)
	assert.Equal(t, []string{"App A", "App C", "App B"}, namesOf(rows))
	assert.Equal(t, []int{3, 2, 1}, sortsOf(t, rows), "positions stay dense from the top")
}

// TestAppReorder_LeavesRowsOutsideTheSubmittedSetInPlace covers the reorder a
// filtered or paginated view sends: only the slots the submitted apps already
// occupied are rewritten, so the rows the admin could not see keep their place.
func TestAppReorder_LeavesRowsOutsideTheSubmittedSetInPlace(t *testing.T) {
	env := newRHITestEnv(t, "8703-sort-subset")
	mountSortRoutes(env)

	// App A is the row the filtered view does not show.
	env.createApp("App A", "8703-app-a", false, 0, 1.0)
	appB := env.createApp("App B", "8703-app-b", false, 0, 1.0)
	appC := env.createApp("App C", "8703-app-c", false, 0, 1.0)
	appD := env.createApp("App D", "8703-app-d", false, 0, 1.0)
	require.Equal(t, []string{"App D", "App C", "App B", "App A"}, namesOf(appRows(t, env)))

	// A view showing the top three rows only, with App B dragged to the front.
	changed, err := runninghub.AppReorder([]uint{appB, appD, appC})
	require.NoError(t, err)
	assert.Equal(t, 3, changed, "the hidden App A must not be rewritten")

	rows := appRows(t, env)
	assert.Equal(t, []string{"App B", "App D", "App C", "App A"}, namesOf(rows))
	assert.Equal(t, []int{4, 3, 2, 1}, sortsOf(t, rows), "App A keeps the last position")
}

// TestAppReorder_RejectsInvalidOrdersWithoutWriting pins the failure paths: an
// empty order, an oversized one and a stale id are all refused, and a refused
// request leaves the stored order untouched.
func TestAppReorder_RejectsInvalidOrdersWithoutWriting(t *testing.T) {
	env := newRHITestEnv(t, "8704-sort-invalid")
	mountSortRoutes(env)

	appA := env.createApp("App A", "8704-app-a", false, 0, 1.0)
	appB := env.createApp("App B", "8704-app-b", false, 0, 1.0)
	before := appRows(t, env)

	_, err := runninghub.AppReorder(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")

	tooMany := make([]uint, 0, 2001)
	for i := 0; i < 2001; i++ {
		tooMany = append(tooMany, uint(i+1))
	}
	_, err = runninghub.AppReorder(tooMany)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "最多支持")

	changed, err := runninghub.AppReorder([]uint{appA, 424242, appB})
	require.Error(t, err, "an app deleted by another admin must fail the whole request")
	assert.Contains(t, err.Error(), "应用不存在")
	assert.Zero(t, changed)

	after := appRows(t, env)
	assert.Equal(t, namesOf(before), namesOf(after))
	assert.Equal(t, sortsOf(t, before), sortsOf(t, after))
}

// legacyApp is the App model exactly as it existed before the sort column was
// added, mapped onto the same `apps` table. Its tags mirror models.go on
// purpose: the only difference has to be the missing Sort field, otherwise
// AutoMigrate would try to change an unrelated column and the test would fail
// for the wrong reason.
type legacyApp struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name               string  `gorm:"type:varchar(191);not null;uniqueIndex"`
	Slug               string  `gorm:"type:varchar(191);index"`
	Kind               string  `gorm:"type:varchar(32);not null;index"`
	UpstreamID         string  `gorm:"type:varchar(191);not null;index"`
	Description        string  `gorm:"type:text"`
	CoverURL           string  `gorm:"type:varchar(768)"`
	Published          bool    `gorm:"index"`
	AdminOnly          bool    `gorm:"index"`
	CategoryID         uint    `gorm:"index;default:0"`
	Site               string  `gorm:"type:varchar(16);index"`
	ParamSchemaText    string  `gorm:"type:text;column:param_schema"`
	PerCallBilling     bool    `gorm:"index"`
	FixedQuotaPerCall  int64   `gorm:"default:0;not null"`
	PerSecondBilling   bool    `gorm:"index"`
	QuotaPerSecond     int64   `gorm:"default:0;not null"`
	SecondsExpr        string  `gorm:"type:varchar(191)"`
	PerCharBilling     bool    `gorm:"index"`
	QuotaPerChar       int64   `gorm:"default:0;not null"`
	CharCountExpr      string  `gorm:"type:varchar(191)"`
	ModelBaseRateRatio float64 `gorm:"default:1.0;not null"`
}

func (legacyApp) TableName() string { return "apps" }

// TestAppSort_MigratesAnExistingTableWithoutReorderingIt covers the upgrade
// path: every install that predates this column has an `apps` table without
// `sort`, and the boot-time AutoMigrate has to add it in a shape all three
// supported databases accept (NOT NULL together with a DEFAULT — without the
// default, ADD COLUMN is illegal on a table that already holds rows). The list
// those installs see must stay the one they saw before, and a new app must still
// open on top of it.
func TestAppSort_MigratesAnExistingTableWithoutReorderingIt(t *testing.T) {
	origDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:rh_sort_legacy?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	t.Cleanup(func() {
		model.DB = origDB
		_ = sqlDB.Close()
	})

	// The table as the previous release created it, with two apps in it.
	require.NoError(t, db.AutoMigrate(&legacyApp{}))
	for i, name := range []string{"Legacy A", "Legacy B"} {
		require.NoError(t, db.Exec(
			"INSERT INTO apps (id, name, kind, upstream_id, published, admin_only, model_base_rate_ratio) VALUES (?, ?, 'ai_app', ?, 1, 0, 1.0)",
			i+1, name, fmt.Sprintf("8707-%d", i+1),
		).Error)
	}

	// The upgrade: the plugin's own models are migrated at boot.
	require.NoError(t, db.AutoMigrate(&runninghub.App{}, &runninghub.AppCategory{}))

	env := &rhITestEnv{t: t, db: db, router: gin.New()}
	mountSortRoutes(env)

	rows := appRows(t, env)
	assert.Equal(t, []string{"Legacy B", "Legacy A"}, namesOf(rows))
	assert.Equal(t, []int{0, 0}, sortsOf(t, rows), "the column arrives with its default")

	env.createApp("New App", "8707-new", false, 0, 1.0)
	assert.Equal(t, []string{"New App", "Legacy B", "Legacy A"}, namesOf(appRows(t, env)))
}

// TestReorderAppsEndpoint_PersistsThePostedOrder drives the HTTP contract the
// page actually calls: the envelope it reads and the order the next list call
// returns. `updated` tells a no-op drag apart from one that landed.
func TestReorderAppsEndpoint_PersistsThePostedOrder(t *testing.T) {
	env := newRHITestEnv(t, "8705-sort-endpoint")
	mountSortRoutes(env)

	appA := env.createApp("App A", "8705-app-a", false, 0, 1.0)
	appB := env.createApp("App B", "8705-app-b", false, 0, 1.0)
	appC := env.createApp("App C", "8705-app-c", false, 0, 1.0)

	w, raw := doJSON(t, env.router, http.MethodPost, reorderRoute, map[string]any{
		"ids": []uint{appB, appA, appC},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", string(raw))
	envelope := parseAPIEnvelope(t, raw)
	require.True(t, envelope.Success, "body: %s", string(raw))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data missing; body=%s", string(raw))
	assert.Equal(t, float64(3), data["updated"])

	assert.Equal(t, []string{"App B", "App A", "App C"}, namesOf(appRows(t, env)))
}

// TestReorderAppsEndpoint_ReportsAStaleOrder covers the failure envelope: a
// reorder naming an app that no longer exists answers success:false with the
// reason, which is what the page turns into its error toast.
func TestReorderAppsEndpoint_ReportsAStaleOrder(t *testing.T) {
	env := newRHITestEnv(t, "8706-sort-stale")
	mountSortRoutes(env)

	appA := env.createApp("App A", "8706-app-a", false, 0, 1.0)
	before := appRows(t, env)

	w, raw := doJSON(t, env.router, http.MethodPost, reorderRoute, map[string]any{
		"ids": []uint{appA, 424242},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", string(raw))
	envelope := parseAPIEnvelope(t, raw)
	assert.False(t, envelope.Success, "body: %s", string(raw))
	assert.Contains(t, envelope.Message, "应用不存在", "the page shows this message")
	assert.Equal(t, namesOf(before), namesOf(appRows(t, env)))
}
