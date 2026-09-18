package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"biosample-cold-custody-tracking/backend/internal/constants"
)

var transferNumberPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{2,49}$`)

type CustodyTransfer struct {
	Base
	SpecimenID     uint                    `gorm:"index;not null" json:"specimenId"`
	Specimen       Specimen                `json:"specimen,omitempty"`
	TransferNo     string                  `gorm:"size:50;uniqueIndex;not null" json:"transferNo"`
	FromCustodian  string                  `gorm:"size:100;not null" json:"fromCustodian"`
	ToCustodian    string                  `gorm:"size:100;not null" json:"toCustodian"`
	FromLocation   string                  `gorm:"size:200;not null" json:"fromLocation"`
	ToLocation     string                  `gorm:"size:200;not null" json:"toLocation"`
	ToContainerID  *uint                   `gorm:"index" json:"toContainerId,omitempty"`
	ToContainer    *StorageContainer       `json:"toContainer,omitempty"`
	ToPosition     string                  `gorm:"size:120" json:"toPosition,omitempty"`
	State          constants.TransferState `gorm:"size:20;index;not null;default:'prepared'" json:"state"`
	PreparedByID   uint                    `gorm:"index;not null" json:"preparedById"`
	PreparedByName string                  `gorm:"size:100;not null" json:"preparedByName"`
	AcceptedByID   *uint                   `gorm:"index" json:"acceptedById,omitempty"`
	AcceptedByName string                  `gorm:"size:100" json:"acceptedByName,omitempty"`
	PreparedAt     time.Time               `gorm:"index;not null" json:"preparedAt"`
	ResolvedAt     *time.Time              `gorm:"index" json:"resolvedAt,omitempty"`
	TemperatureC   *float64                `gorm:"type:numeric(6,2)" json:"temperatureC,omitempty"`
	Reason         string                  `gorm:"size:1000" json:"reason,omitempty"`
}

func (t *CustodyTransfer) Normalize() {
	t.TransferNo = strings.ToUpper(strings.TrimSpace(t.TransferNo))
	t.FromCustodian = strings.TrimSpace(t.FromCustodian)
	t.ToCustodian = strings.TrimSpace(t.ToCustodian)
	t.FromLocation = strings.TrimSpace(t.FromLocation)
	t.ToLocation = strings.TrimSpace(t.ToLocation)
	t.ToPosition = strings.TrimSpace(t.ToPosition)
	t.PreparedByName = strings.TrimSpace(t.PreparedByName)
	t.AcceptedByName = strings.TrimSpace(t.AcceptedByName)
	t.Reason = strings.TrimSpace(t.Reason)
	if t.State == "" {
		t.State = constants.TransferStatePrepared
	}
}

func (t CustodyTransfer) Validate() error {
	if t.SpecimenID == 0 {
		return fmt.Errorf("specimen is required")
	}
	if !transferNumberPattern.MatchString(t.TransferNo) {
		return fmt.Errorf("transfer number must contain 3-50 uppercase letters, numbers or hyphens")
	}
	if !t.State.Valid() {
		return fmt.Errorf("unsupported transfer state: %s", t.State)
	}
	if length := len([]rune(t.FromCustodian)); length < 2 || length > 100 {
		return fmt.Errorf("from custodian must contain 2-100 characters")
	}
	if length := len([]rune(t.ToCustodian)); length < 2 || length > 100 {
		return fmt.Errorf("to custodian must contain 2-100 characters")
	}
	if t.FromCustodian == t.ToCustodian {
		return fmt.Errorf("custody transfer requires different custodians")
	}
	if t.FromLocation == "" || len([]rune(t.FromLocation)) > 200 {
		return fmt.Errorf("from location must contain 1-200 characters")
	}
	if t.ToLocation == "" || len([]rune(t.ToLocation)) > 200 {
		return fmt.Errorf("to location must contain 1-200 characters")
	}
	if t.PreparedByID == 0 || t.PreparedByName == "" || t.PreparedAt.IsZero() {
		return fmt.Errorf("preparer identity and time are required")
	}
	if t.TemperatureC != nil && (*t.TemperatureC < -210 || *t.TemperatureC > 40) {
		return fmt.Errorf("temperature must be between -210 and 40 Celsius")
	}
	if len([]rune(t.ToPosition)) > 120 || len([]rune(t.Reason)) > 1000 {
		return fmt.Errorf("transfer position or reason is too long")
	}
	if t.State == constants.TransferStatePrepared {
		if t.ResolvedAt != nil || t.AcceptedByID != nil || t.AcceptedByName != "" {
			return fmt.Errorf("prepared transfer cannot contain resolution metadata")
		}
		return nil
	}
	if t.ResolvedAt == nil || t.AcceptedByID == nil || *t.AcceptedByID == 0 || t.AcceptedByName == "" {
		return fmt.Errorf("resolved transfer requires resolver identity and time")
	}
	if t.State == constants.TransferStateAccepted {
		if t.ToContainerID == nil || *t.ToContainerID == 0 || t.ToPosition == "" {
			return fmt.Errorf("accepted transfer requires target container and position")
		}
	} else if t.Reason == "" {
		return fmt.Errorf("rejected or cancelled transfer requires a reason")
	}
	return nil
}

func (t CustodyTransfer) CanResolveTo(next constants.TransferState) bool {
	return t.State.CanResolveTo(next)
}

func (t CustodyTransfer) MovesSpecimen() bool {
	return t.State == constants.TransferStateAccepted
}
