// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package dbsqlc

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type AssociatedAccountType string

const (
	AssociatedAccountTypeToken AssociatedAccountType = "token"
)

func (e *AssociatedAccountType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = AssociatedAccountType(s)
	case string:
		*e = AssociatedAccountType(s)
	default:
		return fmt.Errorf("unsupported scan type for AssociatedAccountType: %T", src)
	}
	return nil
}

type NullAssociatedAccountType struct {
	AssociatedAccountType AssociatedAccountType `json:"associated_account_type"`
	Valid                 bool                  `json:"valid"` // Valid is true if AssociatedAccountType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullAssociatedAccountType) Scan(value interface{}) error {
	if value == nil {
		ns.AssociatedAccountType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.AssociatedAccountType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullAssociatedAccountType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.AssociatedAccountType), nil
}

type AuthProviderType string

const (
	AuthProviderTypeGithub  AuthProviderType = "github"
	AuthProviderTypeDiscord AuthProviderType = "discord"
	AuthProviderTypeGoogle  AuthProviderType = "google"
)

func (e *AuthProviderType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = AuthProviderType(s)
	case string:
		*e = AuthProviderType(s)
	default:
		return fmt.Errorf("unsupported scan type for AuthProviderType: %T", src)
	}
	return nil
}

type NullAuthProviderType struct {
	AuthProviderType AuthProviderType `json:"auth_provider_type"`
	Valid            bool             `json:"valid"` // Valid is true if AuthProviderType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullAuthProviderType) Scan(value interface{}) error {
	if value == nil {
		ns.AuthProviderType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.AuthProviderType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullAuthProviderType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.AuthProviderType), nil
}

type EventType string

const (
	EventTypeTransfer     EventType = "transfer"
	EventTypeMint         EventType = "mint"
	EventTypeBurn         EventType = "burn"
	EventTypeCloseAccount EventType = "close_account"
)

func (e *EventType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = EventType(s)
	case string:
		*e = EventType(s)
	default:
		return fmt.Errorf("unsupported scan type for EventType: %T", src)
	}
	return nil
}

type NullEventType struct {
	EventType EventType `json:"event_type"`
	Valid     bool      `json:"valid"` // Valid is true if EventType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullEventType) Scan(value interface{}) error {
	if value == nil {
		ns.EventType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.EventType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullEventType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.EventType), nil
}

type SyncWalletRequestStatus string

const (
	SyncWalletRequestStatusQueued               SyncWalletRequestStatus = "queued"
	SyncWalletRequestStatusFetchingTransactions SyncWalletRequestStatus = "fetching_transactions"
	SyncWalletRequestStatusParsingEvents        SyncWalletRequestStatus = "parsing_events"
)

func (e *SyncWalletRequestStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = SyncWalletRequestStatus(s)
	case string:
		*e = SyncWalletRequestStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for SyncWalletRequestStatus: %T", src)
	}
	return nil
}

type NullSyncWalletRequestStatus struct {
	SyncWalletRequestStatus SyncWalletRequestStatus `json:"sync_wallet_request_status"`
	Valid                   bool                    `json:"valid"` // Valid is true if SyncWalletRequestStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullSyncWalletRequestStatus) Scan(value interface{}) error {
	if value == nil {
		ns.SyncWalletRequestStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.SyncWalletRequestStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullSyncWalletRequestStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.SyncWalletRequestStatus), nil
}

type Account struct {
	ID                   int32              `json:"id"`
	SelectedAuthProvider AuthProviderType   `json:"selected_auth_provider"`
	Email                string             `json:"email"`
	CreatedAt            pgtype.Timestamptz `json:"created_at"`
	UpdatedAt            pgtype.Timestamptz `json:"updated_at"`
}

type Address struct {
	ID        int32              `json:"id"`
	Value     string             `json:"value"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

type AssociatedAccount struct {
	ID              int32                 `json:"id"`
	WalletID        int32                 `json:"wallet_id"`
	AddressID       int32                 `json:"address_id"`
	LastSignatureID pgtype.Int4           `json:"last_signature_id"`
	Type            AssociatedAccountType `json:"type"`
	CreatedAt       pgtype.Timestamptz    `json:"created_at"`
	UpdatedAt       pgtype.Timestamptz    `json:"updated_at"`
}

type Auth struct {
	AccountID  int32              `json:"account_id"`
	ProviderID string             `json:"provider_id"`
	PType      AuthProviderType   `json:"p_type"`
	Username   string             `json:"username"`
	AvatarUrl  string             `json:"avatar_url"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"`
}

type InnerInstruction struct {
	SignatureID      int32              `json:"signature_id"`
	IxIndex          int16              `json:"ix_index"`
	Index            int16              `json:"index"`
	ProgramAccountID int32              `json:"program_account_id"`
	AccountsIds      []int32            `json:"accounts_ids"`
	Data             string             `json:"data"`
	CreatedAt        pgtype.Timestamptz `json:"created_at"`
	UpdatedAt        pgtype.Timestamptz `json:"updated_at"`
}

type Instruction struct {
	SignatureID      int32              `json:"signature_id"`
	Index            int16              `json:"index"`
	IsKnown          bool               `json:"is_known"`
	ProgramAccountID int32              `json:"program_account_id"`
	AccountsIds      []int32            `json:"accounts_ids"`
	Data             string             `json:"data"`
	CreatedAt        pgtype.Timestamptz `json:"created_at"`
	UpdatedAt        pgtype.Timestamptz `json:"updated_at"`
}

type InstructionEvent struct {
	SignatureID int32              `json:"signature_id"`
	IxIndex     int16              `json:"ix_index"`
	Index       int16              `json:"index"`
	Type        EventType          `json:"type"`
	Data        []byte             `json:"data"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
}

type Session struct {
	ID        pgtype.UUID        `json:"id"`
	AccountID int32              `json:"account_id"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at"`
}

type Signature struct {
	ID        int32              `json:"id"`
	Value     string             `json:"value"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

type SyncWalletRequest struct {
	ID        int32                   `json:"id"`
	WalletID  int32                   `json:"wallet_id"`
	Status    SyncWalletRequestStatus `json:"status"`
	CreatedAt pgtype.Timestamptz      `json:"created_at"`
	UpdatedAt pgtype.Timestamptz      `json:"updated_at"`
}

type Transaction struct {
	ID                    int32              `json:"id"`
	SignatureID           int32              `json:"signature_id"`
	AccountsIds           []int32            `json:"accounts_ids"`
	Timestamp             pgtype.Timestamptz `json:"timestamp"`
	TimestampGranularized pgtype.Timestamptz `json:"timestamp_granularized"`
	Slot                  int64              `json:"slot"`
	BlockIndex            pgtype.Int4        `json:"block_index"`
	Logs                  []string           `json:"logs"`
	Err                   bool               `json:"err"`
	Fee                   int64              `json:"fee"`
	CreatedAt             pgtype.Timestamptz `json:"created_at"`
	UpdatedAt             pgtype.Timestamptz `json:"updated_at"`
}

type ViewInnerInstruction struct {
	SignatureID       int32  `json:"signature_id"`
	IxIndex           int16  `json:"ix_index"`
	InnerInstructions []byte `json:"inner_instructions"`
}

type ViewInstruction struct {
	SignatureID  int32  `json:"signature_id"`
	Instructions []byte `json:"instructions"`
}

type Wallet struct {
	ID                 int32              `json:"id"`
	AccountID          int32              `json:"account_id"`
	LastSignatureID    pgtype.Int4        `json:"last_signature_id"`
	AddressID          int32              `json:"address_id"`
	Label              pgtype.Text        `json:"label"`
	Signatures         int32              `json:"signatures"`
	AssociatedAccounts int32              `json:"associated_accounts"`
	CreatedAt          pgtype.Timestamptz `json:"created_at"`
	UpdatedAt          pgtype.Timestamptz `json:"updated_at"`
}

type WalletToSignature struct {
	ID          pgtype.UUID        `json:"id"`
	WalletID    int32              `json:"wallet_id"`
	SignatureID int32              `json:"signature_id"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
}

type WithTimestamp struct {
	TableName string `json:"table_name"`
}
