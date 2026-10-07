package world

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Store errors. Each one maps to exactly one contract code in errors.go; HTTP
// statuses and envelope codes are decided there, not here.
var (
	// ErrProjectNotFound is returned when a project id does not exist.
	ErrProjectNotFound = errors.New("world: project not found")
	// ErrSnapshotNotFound is returned when a project exists but has no snapshot
	// at the requested version.
	ErrSnapshotNotFound = errors.New("world: snapshot not found")
	// ErrNoCurrentVersion is returned when a project's current_version has no
	// matching snapshot row — a broken invariant, not a client mistake.
	ErrNoCurrentVersion = errors.New("world: project current_version has no snapshot")
)

// db exposes the host connection to the store layer only; controllers and op
// handlers read and write through these functions, never through GORM.
func db() *gorm.DB { return model.DB }

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

// WorldProjectCreate inserts one project owned by userID and returns its view.
// Ownership is a parameter rather than a field of the input struct so a caller
// cannot forget to set it.
func WorldProjectCreate(userID int, name string, novelTitle string) (*WorldProject, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("world: project name is empty")
	}
	if len([]rune(name)) > maxProjectNameLen {
		return nil, fmt.Errorf("world: project name exceeds %d characters", maxProjectNameLen)
	}
	if len([]rune(novelTitle)) > maxNovelTitleLen {
		return nil, fmt.Errorf("world: novel title exceeds %d characters", maxNovelTitleLen)
	}
	if userID <= 0 {
		return nil, fmt.Errorf("world: invalid owner id %d", userID)
	}

	row := &WorldProject{
		UserID:         userID,
		Name:           name,
		NovelTitle:     strings.TrimSpace(novelTitle),
		CurrentVersion: 0,
		Status:         ProjectStatusActive,
	}
	if err := db().Create(row).Error; err != nil {
		return nil, fmt.Errorf("world: create project: %w", err)
	}
	return row, nil
}

// WorldProjectGetOwned returns the project when it exists AND belongs to
// userID. A foreign project is reported as ErrProjectNotFound on purpose: "this
// id exists but is not yours" is itself information the caller has no business
// reading.
func WorldProjectGetOwned(projectID uint, userID int) (*WorldProject, error) {
	if projectID == 0 || userID <= 0 {
		return nil, ErrProjectNotFound
	}
	var row WorldProject
	err := db().Where("id = ? AND user_id = ?", projectID, userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("world: load project %d: %w", projectID, err)
	}
	return &row, nil
}

// WorldProjectGet returns a project regardless of owner. It exists for the
// admin face (docs/23 §7: operators must be able to investigate "what happened
// to this user's world"); the user face must call WorldProjectGetOwned.
func WorldProjectGet(projectID uint) (*WorldProject, error) {
	if projectID == 0 {
		return nil, ErrProjectNotFound
	}
	var row WorldProject
	err := db().Where("id = ?", projectID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("world: load project %d: %w", projectID, err)
	}
	return &row, nil
}

// WorldProjectSetCurrentVersion advances the project's version pointer. It is
// only called together with a snapshot insert (WorldSnapshotAppend), so the
// pointer never names a version that does not exist.
func WorldProjectSetCurrentVersion(projectID uint, version int64) error {
	if projectID == 0 {
		return ErrProjectNotFound
	}
	return worldProjectSetCurrentVersion(db(), projectID, version)
}

func worldProjectSetCurrentVersion(tx *gorm.DB, projectID uint, version int64) error {
	res := tx.Model(&WorldProject{}).Where("id = ?", projectID).
		Updates(map[string]any{"current_version": version, "updated_at": nowStamp()})
	if res.Error != nil {
		return fmt.Errorf("world: set current version of project %d: %w", projectID, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// WorldProjectListByUser returns one page of the projects a user owns, newest
// first. It is the list `world.get`-less clients and the admin face both need.
func WorldProjectListByUser(userID int, offset int, limit int) ([]WorldProject, int64, error) {
	if userID <= 0 {
		return nil, 0, nil
	}
	base := db().Model(&WorldProject{}).Where("user_id = ?", userID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("world: count projects of user %d: %w", userID, err)
	}
	var rows []WorldProject
	if err := base.Order("id desc").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("world: list projects of user %d: %w", userID, err)
	}
	return rows, total, nil
}

// ---------------------------------------------------------------------------
// Snapshots (the version history)
// ---------------------------------------------------------------------------

// WorldSnapshotAppend writes doc as the next version of projectID and returns
// the new snapshot. Versioning is per project and strictly increasing, matching
// 工具1's `save_document` (INSERT with version = MAX(version) + 1).
//
// ⚠ doc is treated as an opaque byte string. It must already be valid JSON —
// it is checked only so the row can be serialized back into a JSON envelope
// later (see WorldSnapshotDocRaw); no MTW rule is evaluated here.
func WorldSnapshotAppend(projectID uint, doc []byte, reason string) (*WorldSnapshot, error) {
	if projectID <= 0 {
		return nil, ErrProjectNotFound
	}
	trimmed := strings.TrimSpace(string(doc))
	if trimmed == "" {
		return nil, fmt.Errorf("world: snapshot document is empty")
	}
	if !isJSONValue(trimmed) {
		return nil, fmt.Errorf("world: snapshot document is not valid JSON")
	}
	if len([]rune(reason)) > maxReasonLen {
		return nil, fmt.Errorf("world: snapshot reason exceeds %d characters", maxReasonLen)
	}

	// ★ One transaction around allocate-version + insert + advance the pointer.
	// The pointer must never name a version that does not exist: a crash between
	// the insert and the pointer update would otherwise leave a project that
	// answers "current version 2" while only v1 is readable.
	var row *WorldSnapshot
	err := db().Transaction(func(tx *gorm.DB) error {
		// Confirm the project exists before allocating a version: a snapshot row
		// whose project is missing is unreachable data.
		var count int64
		if err := tx.Model(&WorldProject{}).Where("id = ?", projectID).Count(&count).Error; err != nil {
			return fmt.Errorf("world: probe project %d: %w", projectID, err)
		}
		if count == 0 {
			return ErrProjectNotFound
		}

		next, err := worldSnapshotNextVersion(tx, projectID)
		if err != nil {
			return err
		}
		created := &WorldSnapshot{
			ProjectID: projectID,
			Version:   next,
			Doc:       trimmed,
			Reason:    reason,
		}
		if err := tx.Create(created).Error; err != nil {
			return fmt.Errorf("world: append snapshot v%d to project %d: %w", next, projectID, err)
		}
		if err := worldProjectSetCurrentVersion(tx, projectID, next); err != nil {
			return err
		}
		row = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// WorldSnapshotNextVersion returns the version the next snapshot will take, i.e.
// current max + 1. A project with no snapshots answers 1.
func WorldSnapshotNextVersion(projectID uint) (int64, error) {
	if projectID <= 0 {
		return 0, ErrProjectNotFound
	}
	return worldSnapshotNextVersion(db(), projectID)
}

func worldSnapshotNextVersion(tx *gorm.DB, projectID uint) (int64, error) {
	var maxVersion int64
	query := tx.Model(&WorldSnapshot{}).
		Select("COALESCE(MAX(version), 0)").
		Where("project_id = ?", projectID)
	if err := query.Scan(&maxVersion).Error; err != nil {
		return 0, fmt.Errorf("world: read max version of project %d: %w", projectID, err)
	}
	return maxVersion + 1, nil
}

// WorldSnapshotGet returns one version of a project's document, with the doc
// still in its stored text form.
func WorldSnapshotGet(projectID uint, version int64) (*WorldSnapshot, error) {
	var row WorldSnapshot
	err := db().Where("project_id = ? AND version = ?", projectID, version).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSnapshotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("world: load snapshot v%d of project %d: %w", version, projectID, err)
	}
	return &row, nil
}

// WorldSnapshotCurrent returns the snapshot named by the project's
// current_version pointer. A project with no versions (current_version = 0)
// answers ErrSnapshotNotFound rather than a database error — "an empty world"
// is a legal state, it just has nothing to read yet.
func WorldSnapshotCurrent(projectID uint) (*WorldSnapshot, error) {
	if projectID == 0 {
		return nil, ErrProjectNotFound
	}
	project, err := WorldProjectGet(projectID)
	if err != nil {
		return nil, err
	}
	if project.CurrentVersion <= 0 {
		return nil, ErrSnapshotNotFound
	}
	snapshot, err := WorldSnapshotGet(projectID, project.CurrentVersion)
	if errors.Is(err, ErrSnapshotNotFound) {
		// The pointer and the history disagree: report it as the invariant
		// breach it is instead of pretending the project is empty.
		return nil, fmt.Errorf("%w (project %d points at v%d)",
			ErrNoCurrentVersion, projectID, project.CurrentVersion)
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// WorldSnapshotDocRaw returns a stored document as the exact bytes that will be
// embedded in a JSON response. A row that is no longer parseable — hand-edited
// in the database, say — is reported as a store error rather than answered with
// a corrupt envelope.
func WorldSnapshotDocRaw(projectID uint, version int64) ([]byte, error) {
	snapshot, err := WorldSnapshotGet(projectID, version)
	if err != nil {
		return nil, err
	}
	if !isJSONValue(snapshot.Doc) {
		return nil, fmt.Errorf("world: stored document v%d of project %d is not valid JSON", version, projectID)
	}
	return []byte(snapshot.Doc), nil
}

// WorldSnapshotHistory lists a project's versions, newest first, as
// (version, reason, createdAt). It is what the admin detail page (docs/23 §7)
// and the client's "另一处改过" reconciliation both read.
func WorldSnapshotHistory(projectID uint) ([]SnapshotVersion, error) {
	var rows []WorldSnapshot
	err := db().Model(&WorldSnapshot{}).
		Select("version", "reason", "created_at").
		Where("project_id = ?", projectID).
		Order("version desc").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("world: list snapshots of project %d: %w", projectID, err)
	}
	out := make([]SnapshotVersion, 0, len(rows))
	for i := range rows {
		out = append(out, SnapshotVersion{
			Version:   rows[i].Version,
			Reason:    rows[i].Reason,
			CreatedAt: rows[i].CreatedAt,
		})
	}
	return out, nil
}

// SnapshotVersion is one row of a project's version history. It carries no
// document: a history listing must stay cheap even for a multi-megabyte world
// (docs/23 §4.2 — the same reason `world.get` is a separate op).
type SnapshotVersion struct {
	Version   int64  `json:"version"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"createdAt"`
}

// isJSONValue is a *shape* probe, not a rule: it answers "would this text
// serialize into a JSON envelope", which is a transport concern Go owns.
// Whether the document is a legal MTW document is decided by
// schema/1.1/validate.mjs and nowhere else (docs/23 §6.4).
func isJSONValue(text string) bool {
	var probe any
	return common.Unmarshal([]byte(text), &probe) == nil
}
