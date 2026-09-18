package model

import (
	"fmt"
	"strings"
	"time"

	"biosample-cold-custody-tracking/backend/internal/constants"
)

type ProtocolReview struct {
	Base
	SpecimenID        uint                     `gorm:"index;not null" json:"specimenId"`
	Specimen          Specimen                 `json:"specimen,omitempty"`
	ProtocolCode      string                   `gorm:"size:80;index;not null" json:"protocolCode"`
	Decision          constants.ReviewDecision `gorm:"size:24;index;not null" json:"decision"`
	ReviewerID        uint                     `gorm:"index;not null" json:"reviewerId"`
	ReviewerName      string                   `gorm:"size:100;not null" json:"reviewerName"`
	ConsentVerified   bool                     `gorm:"not null" json:"consentVerified"`
	ScopeVerified     bool                     `gorm:"not null" json:"scopeVerified"`
	RetentionUntil    *time.Time               `gorm:"index" json:"retentionUntil,omitempty"`
	DocumentObjectKey string                   `gorm:"size:512" json:"documentObjectKey,omitempty"`
	Notes             string                   `gorm:"size:2000" json:"notes"`
	ReviewedAt        time.Time                `gorm:"index;not null" json:"reviewedAt"`
}

func (r *ProtocolReview) Normalize() {
	r.ProtocolCode = strings.ToUpper(strings.TrimSpace(r.ProtocolCode))
	r.ReviewerName = strings.TrimSpace(r.ReviewerName)
	r.DocumentObjectKey = strings.TrimSpace(r.DocumentObjectKey)
	r.Notes = strings.TrimSpace(r.Notes)
}

func (r ProtocolReview) Validate() error {
	if r.SpecimenID == 0 {
		return fmt.Errorf("specimen is required")
	}
	if !protocolCodePattern.MatchString(r.ProtocolCode) {
		return fmt.Errorf("protocol code has an invalid format")
	}
	if !r.Decision.Valid() {
		return fmt.Errorf("unsupported protocol review decision: %s", r.Decision)
	}
	if r.ReviewerID == 0 || r.ReviewerName == "" {
		return fmt.Errorf("reviewer identity is required")
	}
	if len([]rune(r.ReviewerName)) > 100 {
		return fmt.Errorf("reviewer name cannot exceed 100 characters")
	}
	if r.ReviewedAt.IsZero() {
		return fmt.Errorf("reviewed time is required")
	}
	if r.Decision == constants.DecisionApproved && (!r.ConsentVerified || !r.ScopeVerified) {
		return fmt.Errorf("approved review requires verified consent and scope")
	}
	if r.RetentionUntil != nil && !r.RetentionUntil.After(r.ReviewedAt) {
		return fmt.Errorf("retention deadline must be after review time")
	}
	if strings.HasPrefix(r.DocumentObjectKey, "/") || strings.Contains(r.DocumentObjectKey, "..") {
		return fmt.Errorf("document object key has an invalid path")
	}
	if len([]rune(r.DocumentObjectKey)) > 512 || len([]rune(r.Notes)) > 2000 {
		return fmt.Errorf("document object key or notes is too long")
	}
	if r.Decision != constants.DecisionApproved && r.Notes == "" {
		return fmt.Errorf("hold or rejected review requires notes")
	}
	return nil
}
