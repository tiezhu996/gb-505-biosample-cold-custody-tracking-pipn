package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrImmutableAuditLog = errors.New("audit log entries are append-only")

type AuditLog struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time `gorm:"index;not null" json:"createdAt"`
	RequestID       string    `gorm:"size:64;index;not null" json:"requestId"`
	ActorID         uint      `gorm:"index;not null" json:"actorId"`
	ActorName       string    `gorm:"size:100;not null" json:"actorName"`
	Action          string    `gorm:"size:80;index;not null" json:"action"`
	EntityType      string    `gorm:"size:80;index;not null" json:"entityType"`
	EntityID        uint      `gorm:"index;not null" json:"entityId"`
	BeforeState     string    `gorm:"type:text;not null" json:"beforeState"`
	AfterState      string    `gorm:"type:text;not null" json:"afterState"`
	BeforeLocation  string    `gorm:"size:240" json:"beforeLocation"`
	AfterLocation   string    `gorm:"size:240" json:"afterLocation"`
	BeforeCustodian string    `gorm:"size:100" json:"beforeCustodian"`
	AfterCustodian  string    `gorm:"size:100" json:"afterCustodian"`
	IPAddress       string    `gorm:"size:64" json:"ipAddress"`
	PreviousHash    string    `gorm:"size:64;not null" json:"previousHash"`
	EntryHash       string    `gorm:"size:64;uniqueIndex;not null" json:"entryHash"`
}

func (a *AuditLog) BeforeUpdate(*gorm.DB) error {
	return ErrImmutableAuditLog
}

func (a *AuditLog) BeforeDelete(*gorm.DB) error {
	return ErrImmutableAuditLog
}

func (a *AuditLog) Seal(previousHash string) {
	a.PreviousHash = previousHash
	a.EntryHash = a.calculateHash()
}

func (a AuditLog) IntegrityValid() bool {
	return a.EntryHash != "" && a.EntryHash == a.calculateHash()
}

func (a AuditLog) calculateHash() string {
	payload := fmt.Sprintf(
		"%s|%d|%s|%d|%s|%s|%s|%d|%s|%s|%s|%s|%s|%s|%s",
		a.PreviousHash,
		a.CreatedAt.UTC().UnixNano(),
		a.RequestID,
		a.ActorID,
		a.ActorName,
		a.Action,
		a.EntityType,
		a.EntityID,
		a.BeforeState,
		a.AfterState,
		a.BeforeLocation,
		a.AfterLocation,
		a.BeforeCustodian,
		a.AfterCustodian,
		a.IPAddress,
	)
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}
