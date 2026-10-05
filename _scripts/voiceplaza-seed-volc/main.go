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
)

func main() {
	csvPath := flag.String("csv", filepath.Join("seedmodel", "火山引擎音色库列表.csv"),
		"CSV file to import")
	mode := flag.String("mode", voice.ImportModeUpsert,
		"import mode: "+voice.ImportModeUpsert+" | "+voice.ImportModeCreate)
	dry := flag.Bool("dry", false, "parse and validate only; every write is rolled back")
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
	if *dry {
		// A dry run still exercises insert/update against the real schema, then
		// throws the whole transaction away.
		model.DB = model.DB.Begin()
		fmt.Println("DRY RUN: all writes will be rolled back")
	}

	result, err := voice.ImportVoices(rows, *mode, warnings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import failed:", err)
		if *dry {
			_ = model.DB.Rollback()
		}
		os.Exit(1)
	}

	if *dry {
		if err := model.DB.Rollback().Error; err != nil {
			fmt.Fprintln(os.Stderr, "rollback failed:", err)
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
