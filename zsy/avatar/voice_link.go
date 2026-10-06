package avatar

import "strings"

// The 形象广场 → 音色广场 link.
//
// A persona names the voice it speaks with by `voice_type` (Avatar.VoiceID); the
// voice catalog owns the actual sample audio. Reads therefore resolve the link
// instead of duplicating the URL onto the persona row, so re-uploading a voice
// sample immediately updates every persona that uses that voice, and a persona
// whose voice has not been imported yet is still a usable row.
//
// The lookup reads the voice catalog's table directly (a narrow projection, not
// the voice package's model): the two plugins must stay independently
// installable, so neither imports the other's Go package, and pulling in the
// whole Voice row would couple this read to that table's full schema.
// voiceCatalogTable is asserted by
// store_test.go/TestResolveVoiceSamples_ReadsTheRealVoiceTable, so a rename
// there fails loudly here instead of silently emptying every sample URL.

// voiceCatalogTable is the table the 音色广场 plugin owns (see
// zsy/voice/models.go Voice.TableName).
const voiceCatalogTable = "zsy_voices"

// VoiceSample is what a persona reports about its linked voice: the display name
// plus the sample audio the plaza serves for it.
type VoiceSample struct {
	Name      string
	AudioURL  string
	AudioName string
}

// voiceSampleRow is the narrow projection read from the voice catalog. The
// column names are bound with explicit `gorm:"column:..."` tags so the mapping
// does not depend on GORM's NamingStrategy, and TableName pins the query to the
// catalog table.
type voiceSampleRow struct {
	VoiceType string `gorm:"column:voice_type"`
	Name      string `gorm:"column:name"`
	AudioURL  string `gorm:"column:audio_url"`
	AudioName string `gorm:"column:audio_name"`
}

func (voiceSampleRow) TableName() string { return voiceCatalogTable }

// catalogVoiceSamples reads one batch of voice ids from the 音色广场 catalog, in
// a single query, and indexes the answer by voice_type. Duplicate ids in the
// input collapse into one row; an id with no catalogued row is simply absent
// from the result.
func catalogVoiceSamples(voiceIDs []string) (map[string]VoiceSample, error) {
	wanted := make([]string, 0, len(voiceIDs))
	seen := make(map[string]struct{}, len(voiceIDs))
	for _, id := range voiceIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		wanted = append(wanted, id)
	}

	samples := make(map[string]VoiceSample, len(wanted))
	if len(wanted) == 0 {
		return samples, nil
	}

	rows := []voiceSampleRow{}
	if err := db().
		Model(&voiceSampleRow{}).
		Select("voice_type", "name", "audio_url", "audio_name").
		Where("voice_type IN ?", wanted).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		samples[row.VoiceType] = VoiceSample{
			Name:      row.Name,
			AudioURL:  row.AudioURL,
			AudioName: row.AudioName,
		}
	}
	return samples, nil
}

// voiceSampleResolverOrDefault picks the resolver a read path should use: an
// explicit one (tests inject a stub) or the real catalog lookup. Keeping the
// default here means every production read reports the linked voice sample
// without each caller remembering to ask for it.
func voiceSampleResolverOrDefault(resolve VoiceSampleResolver) VoiceSampleResolver {
	if resolve != nil {
		return resolve
	}
	return catalogVoiceSamples
}
