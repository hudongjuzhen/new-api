// One-off seeder: inserts a Voice Plaza CSV (the Volcengine voice library
// exported to seedmodel/火山引擎音色库列表.csv) into the zsy_voices table through
// the plugin's own CSV import path, so header aliases, per-row validation and the
// upsert semantics behave exactly like POST /dashboard/zsy/voice/import.
//
// Usage:
//
//	go run ./_scripts/voiceplaza-seed-volc -dry           # parse + validate, no writes
//	go run ./_scripts/voiceplaza-seed-volc                # upsert by name (default)
//	go run ./_scripts/voiceplaza-seed-volc -mode=create   # only create new names
//	go run ./_scripts/voiceplaza-seed-volc -csv=path.csv
//
// SQL_DSN comes from the process env or .env, so the target database is chosen by
// the environment: run it next to the deployment's .env to seed that database.
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
	"github.com/QuantumNous/new-api/zsy/voice"
	"gorm.io/gorm"
)

func main() {
	csvPath := flag.String("csv", filepath.Join("seedmodel", "火山引擎音色库列表.csv"),
		"CSV file to import")
	mode := flag.String("mode", voice.ImportModeUpsert,
		"import mode: "+voice.ImportModeUpsert+" | "+voice.ImportModeCreate)
	dry := flag.Bool("dry", false, "parse and validate only; every write is rolled back")
	replace := flag.Bool("replace", false, "delete every stored voice before importing (clean rebuild)")
	stats := flag.Bool("stats", false, "print the catalogue counts per filter and exit")
	sample := flag.Int("sample", 3, "how many parsed rows to print")
	flag.Parse()

	_ = godotenv.Load(".env")
	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "init db failed:", err)
		os.Exit(1)
	}

	// The plugin registers zsy_voices with extcore, so InitDB already migrated it.
	// Repeating it here keeps the seeder usable against a database that was last
	// touched by a build from before a column was added.
	if err := model.DB.AutoMigrate(&voice.Voice{}); err != nil {
		fmt.Fprintln(os.Stderr, "migrate zsy_voices failed:", err)
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

	rows, warnings, err := voice.ParseVoicesCSV(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse csv failed:", err)
		os.Exit(1)
	}
	fmt.Printf("parsed %d rows from %s\n", len(rows), *csvPath)
	for _, row := range rows[:min(*sample, len(rows))] {
		fmt.Printf("  row %-4d %-24s %-38s gender=%-6s lang=%-3s scenes=%v avatar=%t audio=%t\n",
			row.Row, row.Create.Name, row.Create.VoiceType, row.Create.Gender,
			row.Create.Language, row.Create.Scenes,
			row.Create.AvatarURL != "", row.Create.AudioURL != "")
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
		// the plaza. Every stored voice goes, including rows this CSV does not know
		// about, which is the point of a clean rebuild.
		deleted := model.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&voice.Voice{})
		if deleted.Error != nil {
			fmt.Fprintln(os.Stderr, "clear failed:", deleted.Error)
			if inTransaction {
				_ = model.DB.Rollback()
			}
			os.Exit(1)
		}
		fmt.Printf("cleared %d stored voices\n", deleted.RowsAffected)
		// Every row is new again, so an upsert run and a create run behave the same.
		*mode = voice.ImportModeCreate
	}

	result, err := voice.ImportVoices(rows, *mode, warnings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import failed:", err)
		if inTransaction {
			_ = model.DB.Rollback()
		}
		os.Exit(1)
	}

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
	var stored, onShelf int64
	if err := model.DB.Model(&voice.Voice{}).Count(&stored).Error; err != nil {
		fmt.Fprintln(os.Stderr, "count failed:", err)
		os.Exit(1)
	}
	if err := model.DB.Model(&voice.Voice{}).Where("enabled = ?", true).Count(&onShelf).Error; err != nil {
		fmt.Fprintln(os.Stderr, "count failed:", err)
		os.Exit(1)
	}
	fmt.Printf("zsy_voices stored=%d (on shelf %d)\n", stored, onShelf)
}

// printStats reports the catalogue through the plugin's own store layer — the
// same code path the public list uses — so the totals below are exactly what
// GET /api/zsy/voice/list?age_range=… answers.
func printStats() {
	onShelf := true
	page := func(q voice.VoiceListQuery) voice.VoiceListResult {
		q.Enabled = &onShelf
		if q.Page < 1 {
			q.Page = 1
		}
		if q.PageSize < 1 {
			q.PageSize = 1
		}
		result, err := voice.VoiceSearch(q)
		if err != nil {
			fmt.Fprintln(os.Stderr, "stats query failed:", err)
			os.Exit(1)
		}
		return result
	}

	fmt.Printf("on-shelf voices: %d\n", page(voice.VoiceListQuery{}).Total)

	fmt.Println("by age_range:")
	for _, ageRange := range []string{
		voice.AgeChild, voice.AgeTeen, voice.AgeYoung, voice.AgeMiddle, voice.AgeSenior,
	} {
		fmt.Printf("  %-7s %3d\n", ageRange, page(voice.VoiceListQuery{AgeRange: ageRange}).Total)
	}

	fmt.Println("by gender:")
	for _, gender := range []string{voice.GenderFemale, voice.GenderMale, voice.GenderNeutral} {
		fmt.Printf("  %-7s %3d\n", gender, page(voice.VoiceListQuery{Gender: gender}).Total)
	}

	fmt.Println("by language:")
	for _, language := range []string{"zh", "en", "ja", "pt", "id", "mx"} {
		fmt.Printf("  %-4s %3d\n", language, page(voice.VoiceListQuery{Language: language}).Total)
	}

	fmt.Println("combined filter age_range=child&gender=female:")
	fmt.Printf("  %d\n", page(voice.VoiceListQuery{
		AgeRange: voice.AgeChild, Gender: voice.GenderFemale,
	}).Total)

	sample := page(voice.VoiceListQuery{AgeRange: voice.AgeSenior, PageSize: 5})
	fmt.Printf("first page of age_range=%s (%d rows):\n", voice.AgeSenior, sample.Total)
	for _, item := range sample.Items {
		fmt.Printf("  %s | %s | %s | %v | avatar=%t\n",
			item.Name, item.VoiceType, item.Language, item.Scenes, item.AvatarURL != "")
	}
}
