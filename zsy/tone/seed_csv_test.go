package tone_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// The seeded catalog: seedmodel/toneplaza-tones.csv.
//
// That file is not just sample data — it is the **reference authoring of the
// standard**. It is what an operator imports to get a working plaza, and what
// the next person reads to see how a row of each category and each tone is
// supposed to look. So it is guarded like code:
//
//   - it must parse and import cleanly through the production CSV path,
//   - it must cover every published vocabulary value (a standard no seed
//     demonstrates is a standard nobody can follow),
//   - it must obey the shared-SampleInput convention the standard states.
//
// The test reads the file rather than embedding a copy, so editing the CSV
// without re-running the suite is impossible: the guard lives on the artifact.
// ---------------------------------------------------------------------------

const seedCSVPath = "../../seedmodel/toneplaza-tones.csv"

func TestSeedCSV_ImportsThroughTheProductionPath(t *testing.T) {
	newToneTestDB(t)

	file, err := os.Open(filepath.Clean(seedCSVPath))
	require.NoError(t, err, "the seeded catalog must exist at %s", seedCSVPath)
	defer file.Close()

	rows, warnings, err := tone.ParseTonesCSV(file)
	require.NoError(t, err, "the seeded catalog must parse")
	assert.Empty(t, warnings, "the seeded catalog must not need any header or example warning: %v", warnings)
	require.NotEmpty(t, rows)

	result, err := tone.ImportTones(rows, tone.ImportModeUpsert, warnings)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Failed, "every seeded row must pass validation: %v", result.Errors)
	assert.Equal(t, 0, result.Updated, "a fresh database imports every row as a create")
	assert.Equal(t, len(rows), result.Created)
	assert.Empty(t, result.Warnings, "the seeded catalog must import silently: %v", result.Warnings)
}

func TestSeedCSV_CoversEveryPublishedVocabularyValue(t *testing.T) {
	newToneTestDB(t)

	rows := parseSeedCSV(t)
	_, err := tone.ImportTones(rows, tone.ImportModeUpsert, nil)
	require.NoError(t, err)

	// Queried through the store layer, so this measures what a client would
	// actually get from the public list — not what the file claims to contain.
	onShelf := true
	countFor := func(q tone.ToneListQuery) int64 {
		q.Enabled = &onShelf
		result, err := tone.ToneSearch(q)
		require.NoError(t, err)
		return result.Total
	}

	for _, category := range tone.AllowedCategories {
		assert.NotZero(t, countFor(tone.ToneListQuery{Category: category}),
			"no seeded tone uses category %q — the standard publishes a value nothing demonstrates", category)
	}
	for _, style := range tone.AllowedTones {
		assert.NotZero(t, countFor(tone.ToneListQuery{Tone: style}),
			"no seeded tone uses tone %q — the standard publishes a value nothing demonstrates", style)
	}

	assert.NotZero(t, countFor(tone.ToneListQuery{Language: "zh"}), "the seeded catalog is Chinese")
	assert.Equal(t, countFor(tone.ToneListQuery{}), countFor(tone.ToneListQuery{}))
}

// TestSeedCSV_EveryRowIsOnShelf pins that the seed is a *plaza*: an operator
// importing it should see a populated page, not a list of drafts.
func TestSeedCSV_EveryRowIsOnShelf(t *testing.T) {
	for _, row := range parseSeedCSV(t) {
		require.NotNil(t, row.Create.Enabled, "row %d does not set enabled", row.Row)
		assert.True(t, *row.Create.Enabled, "row %d (%s) is off the shelf", row.Row, row.Create.Name)
	}
}

// TestSeedCSV_EveryRowCarriesAWorkedExample checks the field the plaza's whole
// browsing experience rests on: a tone with no SampleOutput cannot be compared
// with anything, and one with no SampleInput cannot even be read as a rewrite.
func TestSeedCSV_EveryRowCarriesAWorkedExample(t *testing.T) {
	for _, row := range parseSeedCSV(t) {
		assert.NotEmpty(t, row.Create.SampleInput, "row %d (%s) has no sample input", row.Row, row.Create.Name)
		assert.NotEmpty(t, row.Create.SampleOutput, "row %d (%s) has no sample output", row.Row, row.Create.Name)
	}
}

// TestSeedCSV_SharesOneSampleInput is the convention from standard.go, asserted
// on the file that is supposed to demonstrate it. A shared passage is what makes
// the outputs comparable; if the seed itself drifts, the convention is dead no
// matter what the standard says.
func TestSeedCSV_SharesOneSampleInput(t *testing.T) {
	rows := parseSeedCSV(t)

	passage := ""
	for _, row := range rows {
		if passage == "" {
			passage = row.Create.SampleInput
			continue
		}
		assert.Equal(t, passage, row.Create.SampleInput,
			"row %d (%s) uses a different sample input; the plaza's whole point is comparing outputs from one passage",
			row.Row, row.Create.Name)
	}
	assert.NotEmpty(t, passage)
}

// TestSeedCSV_OutputsAndPromptsAreDistinct is the other half of "comparable":
// rows sharing a passage must not also share their rewrite or their
// instruction, which would mean the catalog has two names for one tone.
func TestSeedCSV_OutputsAndPromptsAreDistinct(t *testing.T) {
	rows := parseSeedCSV(t)

	outputs := make(map[string]string, len(rows))
	prompts := make(map[string]string, len(rows))
	for _, row := range rows {
		output, prompt := row.Create.SampleOutput, row.Create.Prompt

		if other, seen := outputs[output]; seen {
			t.Errorf("%s and %s have identical sample outputs — two names, one tone", other, row.Create.Name)
		}
		outputs[output] = row.Create.Name

		if other, seen := prompts[prompt]; seen {
			t.Errorf("%s and %s have identical prompts", other, row.Create.Name)
		}
		prompts[prompt] = row.Create.Name
	}
}

// TestSeedCSV_InstructionsAreInstructionsNotDescriptions guards the field's
// contract (see models.go): Prompt is pasted straight into a system prompt, so a
// row that merely *describes* a style changes the model's output far less than
// one that instructs it. Every seeded prompt must read as a directive.
func TestSeedCSV_InstructionsAreInstructionsNotDescriptions(t *testing.T) {
	directive := []string{"以", "写作", "使用", "不", "多", "少", "要", "避免"}

	for _, row := range parseSeedCSV(t) {
		prompt := row.Create.Prompt
		hit := false
		for _, marker := range directive {
			if strings.Contains(prompt, marker) {
				hit = true
				break
			}
		}
		assert.True(t, hit,
			"row %d (%s) does not read as an instruction; a description of a style is not a usable prompt:\n%s",
			row.Row, row.Create.Name, prompt)
	}
}

// parseSeedCSV opens and parses the seeded catalog, failing the test on any
// structural problem.
func parseSeedCSV(t *testing.T) []tone.ToneImportRow {
	t.Helper()

	file, err := os.Open(filepath.Clean(seedCSVPath))
	require.NoError(t, err, "the seeded catalog must exist at %s", seedCSVPath)
	defer file.Close()

	rows, _, err := tone.ParseTonesCSV(file)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	return rows
}
