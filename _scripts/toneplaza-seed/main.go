// Seeder for the 文风广场 (Tone Plaza).
//
// It inserts seedmodel/toneplaza-tones.csv into the zsy_tones table through the
// plugin's own CSV import path, so header aliases, per-row validation and the
// upsert semantics behave exactly like POST /dashboard/zsy/tone/import. Seeding
// through the production path is the point: if the file and the API ever
// disagree, this command fails the same way the import button would, instead of
// writing rows the API could never have accepted.
//
// Usage:
//
//	go run ./_scripts/toneplaza-seed -dry           # parse + validate, no writes
//	go run ./_scripts/toneplaza-seed                # upsert by name (default)
//	go run ./_scripts/toneplaza-seed -mode=create   # only create new names
//	go run ./_scripts/toneplaza-seed -replace       # clear the plaza, rebuild it
//	go run ./_scripts/toneplaza-seed -stats         # report the stored catalogue
//	go run ./_scripts/toneplaza-seed -csv=path.csv
//
// SQL_DSN comes from the process env or .env, so the target database is chosen by
// the environment: run it next to the deployment's .env to seed that database.
//
// ⚠ The seeded catalog is also the reference authoring of the 文风标准 (see
// zsy/tone/standard.go): it is expected to cover every published vocabulary
// value. This command verifies that after importing and fails if a value has no
// example — a standard with no seed demonstrating it is a standard nobody can
// follow. Pass -allow-partial when seeding a deliberately reduced catalogue.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/zsy/tone"
	"gorm.io/gorm"
)

func main() {
	csvPath := flag.String("csv", filepath.Join("seedmodel", "toneplaza-tones.csv"),
		"CSV file to import")
	mode := flag.String("mode", tone.ImportModeUpsert,
		"import mode: "+tone.ImportModeUpsert+" | "+tone.ImportModeCreate)
	dry := flag.Bool("dry", false, "parse and validate only; every write is rolled back")
	replace := flag.Bool("replace", false, "delete every stored tone before importing (clean rebuild)")
	stats := flag.Bool("stats", false, "print the catalogue counts per filter and exit")
	sample := flag.Int("sample", 3, "how many parsed rows to print")
	allowPartial := flag.Bool("allow-partial", false,
		"do not fail when the imported catalogue misses a published vocabulary value")
	flag.Parse()

	_ = godotenv.Load(".env")
	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "init db failed:", err)
		os.Exit(1)
	}

	// The plugin registers zsy_tones with extcore, so InitDB already migrated it.
	// Repeating it here keeps the seeder usable against a database that was last
	// touched by a build from before a column was added.
	if err := model.DB.AutoMigrate(&tone.Tone{}); err != nil {
		fmt.Fprintln(os.Stderr, "migrate zsy_tones failed:", err)
		os.Exit(1)
	}

	if *stats {
		printStats()
		return
	}

	file, err := os.Open(*csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open csv failed:", err)
		os.Exit(1)
	}
	defer file.Close()

	rows, warnings, err := tone.ParseTonesCSV(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse csv failed:", err)
		os.Exit(1)
	}
	fmt.Printf("parsed %d rows from %s\n", len(rows), *csvPath)
	for _, row := range rows[:min(*sample, len(rows))] {
		fmt.Printf("  row %-3d %-22s category=%-10s tone=%-9s lang=%-3s scenes=%v example=%t/%t\n",
			row.Row, row.Create.Name, row.Create.Category, row.Create.Tone,
			row.Create.Language, row.Create.Scenes,
			row.Create.SampleInput != "", row.Create.SampleOutput != "")
	}
	for _, warning := range warnings {
		fmt.Println("  WARN", warning)
	}

	originalDB := model.DB
	// A clean rebuild runs inside one transaction so readers see either the old
	// catalogue or the new one: emptying the plaza and refilling it row by row
	// would otherwise leave the public list short for minutes.
	inTransaction := *dry || *replace
	if inTransaction {
		// Delete/insert/update all run against the real schema, then either commit
		// (-replace) or get thrown away (-dry).
		model.DB = model.DB.Begin()
		if *dry {
			fmt.Println("DRY RUN: all writes will be rolled back")
		} else {
			fmt.Println("REPLACE: clearing and rebuilding inside one transaction")
		}
	}

	if *replace {
		// Hard delete: the plugin has no soft-delete column, so this really empties
		// the plaza. Every stored tone goes, including rows this CSV does not know
		// about, which is the point of a clean rebuild.
		deleted := model.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&tone.Tone{})
		if deleted.Error != nil {
			fmt.Fprintln(os.Stderr, "clear failed:", deleted.Error)
			if inTransaction {
				_ = model.DB.Rollback()
			}
			os.Exit(1)
		}
		fmt.Printf("cleared %d stored tones\n", deleted.RowsAffected)
		// Every row is new again, so an upsert run and a create run behave the same.
		*mode = tone.ImportModeCreate
	}

	result, err := tone.ImportTones(rows, *mode, warnings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import failed:", err)
		if inTransaction {
			_ = model.DB.Rollback()
		}
		os.Exit(1)
	}

	// Coverage is checked *inside* the transaction, so a dry run reports the same
	// verdict a real run would and the rollback below has still happened.
	missing := missingVocabularyValues()

	if inTransaction {
		if *dry {
			if err := model.DB.Rollback().Error; err != nil {
				fmt.Fprintln(os.Stderr, "rollback failed:", err)
				os.Exit(1)
			}
			fmt.Println("dry run finished; every write rolled back")
		} else if err := model.DB.Commit().Error; err != nil {
			fmt.Fprintln(os.Stderr, "commit failed:", err)
			os.Exit(1)
		}
		model.DB = originalDB
	}

	fmt.Printf("mode=%s total=%d created=%d updated=%d failed=%d\n",
		*mode, result.Total, result.Created, result.Updated, result.Failed)
	for _, warning := range result.Warnings {
		fmt.Println("  WARN", warning)
	}
	for _, rowError := range result.Errors {
		fmt.Printf("  FAIL row %d %q: %s\n", rowError.Row, rowError.Name, rowError.Message)
	}

	// The stored state after this run: in a dry run it must be unchanged, which is
	// what proves the rollback above was effective.
	fmt.Printf("zsy_tones stored=%d (on shelf %d)\n", countAll(), countOnShelf())

	if len(missing) > 0 {
		fmt.Printf("\n⚠ 文风标准覆盖不全，缺少示例的取值: %v\n", missing)
		fmt.Println("  种子数据是标准的示范：每个已发布的取值都该至少有一条。")
		if !*allowPartial {
			fmt.Println("  要导入一份有意精简的目录，请加 -allow-partial。")
			os.Exit(1)
		}
	} else {
		fmt.Println("\n✓ 文风标准覆盖完整：" + fmt.Sprintf("%d 个类别 / %d 种语气都有示例",
			len(tone.AllowedCategories), len(tone.AllowedTones)))
	}
}

// missingVocabularyValues lists the published values the on-shelf catalogue has
// no example for. It reads through the store layer (the same path the public
// list uses), so it measures what a client would actually receive.
//
// Only on-shelf rows count: a value demonstrated solely by a draft is not
// visible in the plaza, which is where an operator looks for guidance.
func missingVocabularyValues() []string {
	missing := make([]string, 0)
	onShelf := true

	countFor := func(q tone.ToneListQuery) int64 {
		q.Enabled = &onShelf
		result, err := tone.ToneSearch(q)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coverage query failed:", err)
			os.Exit(1)
		}
		return result.Total
	}

	for _, category := range tone.AllowedCategories {
		if countFor(tone.ToneListQuery{Category: category}) == 0 {
			missing = append(missing, "category="+category)
		}
	}
	for _, style := range tone.AllowedTones {
		if countFor(tone.ToneListQuery{Tone: style}) == 0 {
			missing = append(missing, "tone="+style)
		}
	}
	return missing
}

func countAll() int64 { return countWhere(nil) }

func countOnShelf() int64 {
	onShelf := true
	return countWhere(&onShelf)
}

func countWhere(enabled *bool) int64 {
	query := model.DB.Model(&tone.Tone{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		fmt.Fprintln(os.Stderr, "count failed:", err)
		os.Exit(1)
	}
	return total
}

// printStats reports the catalogue through the plugin's own store layer — the
// same code path the public list uses — so the totals below are exactly what
// GET /api/zsy/tone/list?category=… answers.
func printStats() {
	onShelf := true
	page := func(q tone.ToneListQuery) tone.ToneListResult {
		q.Enabled = &onShelf
		if q.Page < 1 {
			q.Page = 1
		}
		if q.PageSize < 1 {
			q.PageSize = 1
		}
		result, err := tone.ToneSearch(q)
		if err != nil {
			fmt.Fprintln(os.Stderr, "stats query failed:", err)
			os.Exit(1)
		}
		return result
	}

	fmt.Printf("on-shelf tones: %d\n", page(tone.ToneListQuery{}).Total)

	fmt.Println("by category:")
	for _, category := range tone.AllowedCategories {
		fmt.Printf("  %-10s %3d\n", category, page(tone.ToneListQuery{Category: category}).Total)
	}

	fmt.Println("by tone:")
	for _, style := range tone.AllowedTones {
		fmt.Printf("  %-9s %3d\n", style, page(tone.ToneListQuery{Tone: style}).Total)
	}

	fmt.Println("by language:")
	for _, language := range []string{"zh", "en", "ja"} {
		fmt.Printf("  %-4s %3d\n", language, page(tone.ToneListQuery{Language: language}).Total)
	}

	fmt.Println("combined filter category=marketing&tone=sharp:")
	fmt.Printf("  %d\n", page(tone.ToneListQuery{
		Category: tone.CategoryMarketing, Tone: tone.ToneSharp,
	}).Total)

	if missing := missingVocabularyValues(); len(missing) > 0 {
		fmt.Printf("standard coverage: INCOMPLETE, missing %v\n", missing)
	} else {
		fmt.Println("standard coverage: complete")
	}

	first := page(tone.ToneListQuery{PageSize: 5})
	fmt.Printf("first page (%d rows):\n", first.Total)
	for _, item := range first.Items {
		fmt.Printf("  %s | %s | %s | %v | example=%t\n",
			item.Name, item.Category, item.Tone, item.Scenes,
			item.SampleInput != "" && item.SampleOutput != "")
	}
}
