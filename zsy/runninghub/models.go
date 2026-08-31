package runninghub

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AppKind categorises the upstream invocation pattern.
type AppKind string

const (
	AppKindAICApp   AppKind = "ai_app"   // POST /openapi/v2/run/ai-app/{appId}
	AppKindWorkflow AppKind = "workflow" // POST /openapi/v2/run/workflow/{id}
	AppKindModel    AppKind = "model"    // POST /openapi/v2/<relative-path>
)

// ParameterFieldType categorises a user-configurable parameter for an app.
type ParameterFieldType string

const (
	FieldTypeText     ParameterFieldType = "text"
	FieldTypeTextarea ParameterFieldType = "textarea"
	FieldTypeNumber   ParameterFieldType = "number"
	FieldTypeImage    ParameterFieldType = "image"
	FieldTypeAudio    ParameterFieldType = "audio"
	FieldTypeVideo    ParameterFieldType = "video"
	FieldTypeSelect   ParameterFieldType = "select"
)

// FieldParam records one user-facing editable parameter on an app or
// workflow node. The (NodeID, FieldName) pair identifies the target when the
// adaptor builds the upstream request body; Label/Placeholder provide the
// user-visible hints.
type FieldParam struct {
	NodeID      string             `json:"nodeId"`
	FieldName   string             `json:"fieldName"`
	Label       string             `json:"label"`
	Type        ParameterFieldType `json:"type"`
	Required    bool               `json:"required"`
	Default     string             `json:"defaultValue,omitempty"`
	Placeholder string             `json:"placeholder,omitempty"`
	Min         *float64           `json:"min,omitempty"`
	Max         *float64           `json:"max,omitempty"`
	Options     []ParameterOption  `json:"options,omitempty"`
}

// ParameterOption is a single entry for FieldTypeSelect parameters.
type ParameterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// App mirrors the new-api level representation of a RunningHub application.
// Upstream identifiers live in Kind + UpstreamID; anything the admin edits
// (display, pricing, parameter schema) is stored here independently.
type App struct {
	ID        uint           `gorm:"primarykey"                                 json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                    json:"-"`

	Name        string  `gorm:"type:varchar(191);not null;uniqueIndex"   json:"name"`
	Slug        string  `gorm:"type:varchar(191);index"                  json:"slug"`
	Kind        AppKind `gorm:"type:varchar(32);not null;index"          json:"kind"`
	UpstreamID  string  `gorm:"type:varchar(191);not null;index"         json:"upstreamId"`
	Description string  `gorm:"type:text"                                json:"description"`
	CoverURL    string  `gorm:"type:varchar(768)"                        json:"coverUrl"`
	Published   bool    `gorm:"index"                                     json:"published"`
	AdminOnly   bool    `gorm:"index"                                     json:"adminOnly"`

	// ChannelID optionally pins the app to a specific RunningHub channel; the
	// admin chooses it in the app form and the submit path locks that channel.
	// When zero, the submit path falls back to the host's model->channel
	// selection (GetChannelForRelay) as before.
	ChannelID int64 `gorm:"index"                                     json:"channelId"`

	// ParamSchema stores the JSON text of the user-facing parameter list. We
	// intentionally persist as TEXT (not JSON column) for cross-DB compatibility
	// with legacy MySQL < 5.7.8 / PostgreSQL < 9.6; see AGENTS.md.
	ParamSchemaText string `gorm:"type:text;column:param_schema"            json:"-"`

	// PerCallBilling mirrors the new-api billing semantics. When true the task
	// is charged flatly at submission and never refunded, matching v1 of the
	// RH pricing model. When false the billing chain supports
	// AdjustBillingOnComplete driven by RH usage.consumeCoins.
	PerCallBilling bool `gorm:"index"                                     json:"perCallBilling"`

	// Price per invocation when PerCallBilling=true. Stored as an integer
	// quota unit (same convention as model prices in the host) to avoid
	// decimal arithmetic drift.
	FixedQuotaPerCall int64 `gorm:"default:0;not null"                       json:"fixedQuotaPerCall"`

	// ModelBaseRateRatio is the multiplier applied against the *channel's*
	// base model price when PerCallBilling is off. 1.0 means "1× standard
	// base rate". Any non-positive ratio is rejected by the validator so the
	// billing chain is always protected.
	ModelBaseRateRatio float64 `gorm:"default:1.0;not null"                     json:"modelBaseRateRatio"`
}

// ParamSchema returns the decoded parameter list. An empty schema is a valid,
// non-error result (apps with zero-user-visible fields are allowed).
func (a *App) ParamSchema() ([]FieldParam, error) {
	if a.ParamSchemaText == "" || a.ParamSchemaText == "null" {
		return []FieldParam{}, nil
	}
	var out []FieldParam
	err := common.UnmarshalJsonStr(a.ParamSchemaText, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetParamSchema persists a parameter list as its JSON text encoding.
func (a *App) SetParamSchema(schema []FieldParam) error {
	if schema == nil {
		a.ParamSchemaText = ""
		return nil
	}
	data, err := common.Marshal(schema)
	if err != nil {
		return err
	}
	a.ParamSchemaText = string(data)
	return nil
}

// AppKeyPool is the per-app keypool, synced from the app's bound channel key
// list (App.ChannelID). This was merged up from the old per-instance keypool
// when the instance concept was removed; pending entries on KeypoolPending
// keep the quota-leak auditing of §7.6 in the dev plan intact.
//
// A key is unique per app (app_id + key): the same RH key may legitimately be
// shared by several apps bound to the same channel.
type AppKeyPool struct {
	ID        uint           `gorm:"primarykey"                                    json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                        json:"-"`

	AppID   uint   `gorm:"not null;uniqueIndex:udx_app_key"              json:"appId"`
	Key     string `gorm:"type:varchar(255);not null;uniqueIndex:udx_app_key" json:"key"`
	Enabled bool   `gorm:"default:true;index"                            json:"enabled"`
	Remark  string `gorm:"type:varchar(768)"                             json:"remark"`
}

// KeypoolPending records one in-flight submit for auditing quota-safety
// invariants (§7.6). Records are soft-deleted after settlement so the admin
// UI can inspect crashes / partial-write scenarios.
type KeypoolPending struct {
	ID        uint           `gorm:"primarykey"                                  json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                      json:"-"`

	PoolID         uint   `gorm:"index;not null"                              json:"poolId"`
	NewApiTaskID   string `gorm:"type:varchar(128);not null;index"           json:"newApiTaskId"`
	UpstreamTaskID string `gorm:"type:varchar(128);index"                     json:"upstreamTaskId"`
	EstimatedQuota int64  `gorm:"default:0;not null"                          json:"estimatedQuota"`
	State          string `gorm:"type:varchar(32);index;default:'pending'"   json:"state"`
	FailureReason  string `gorm:"type:text"                                   json:"failureReason,omitempty"`
}
