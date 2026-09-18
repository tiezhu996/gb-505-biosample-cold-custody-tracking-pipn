package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"biosample-cold-custody-tracking/backend/internal/constants"
)

var accessionNumberPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{2,49}$`)
var protocolCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{2,79}$`)

type Specimen struct {
	Base
	AccessionNo        string                  `gorm:"size:50;uniqueIndex;not null" json:"accessionNo"`
	SampleType         string                  `gorm:"size:100;index;not null" json:"sampleType"`
	SubjectCode        string                  `gorm:"size:80;index;not null" json:"subjectCode"`
	ProtocolCode       string                  `gorm:"size:80;index;not null" json:"protocolCode"`
	State              constants.SpecimenState `gorm:"size:20;index;not null;default:'received'" json:"state"`
	StorageContainerID *uint                   `gorm:"index" json:"storageContainerId,omitempty"`
	StorageContainer   *StorageContainer       `json:"storageContainer,omitempty"`
	Position           string                  `gorm:"size:120" json:"position,omitempty"`
	VolumeML           float64                 `gorm:"type:numeric(12,3);not null" json:"volumeMl"`
	AliquotCount       int                     `gorm:"not null;default:0" json:"aliquotCount"`
	CurrentCustodian   string                  `gorm:"size:100;not null" json:"currentCustodian"`
	ReceivedAt         time.Time               `gorm:"index;not null" json:"receivedAt"`
	ExpiresAt          *time.Time              `gorm:"index" json:"expiresAt,omitempty"`
	Notes              string                  `gorm:"size:1000" json:"notes,omitempty"`
	Transfers          []CustodyTransfer       `json:"transfers,omitempty"`
	ProtocolReviews    []ProtocolReview        `json:"protocolReviews,omitempty"`
}

func (s *Specimen) Normalize() {
	s.AccessionNo = strings.ToUpper(strings.TrimSpace(s.AccessionNo))
	s.SampleType = strings.TrimSpace(s.SampleType)
	s.SubjectCode = strings.ToUpper(strings.TrimSpace(s.SubjectCode))
	s.ProtocolCode = strings.ToUpper(strings.TrimSpace(s.ProtocolCode))
	s.Position = strings.TrimSpace(s.Position)
	s.CurrentCustodian = strings.TrimSpace(s.CurrentCustodian)
	s.Notes = strings.TrimSpace(s.Notes)
	if s.State == "" {
		s.State = constants.SpecimenStateReceived
	}
}

func (s Specimen) Validate() error {
	if !accessionNumberPattern.MatchString(s.AccessionNo) {
		return fmt.Errorf("accession number must contain 3-50 uppercase letters, numbers or hyphens")
	}
	if length := len([]rune(s.SampleType)); length < 2 || length > 100 {
		return fmt.Errorf("sample type must contain 2-100 characters")
	}
	if length := len([]rune(s.SubjectCode)); length < 3 || length > 80 {
		return fmt.Errorf("subject code must contain 3-80 characters")
	}
	if !protocolCodePattern.MatchString(s.ProtocolCode) {
		return fmt.Errorf("protocol code has an invalid format")
	}
	if !s.State.Valid() {
		return fmt.Errorf("unsupported specimen state: %s", s.State)
	}
	if s.VolumeML <= 0 || s.VolumeML > 100000 {
		return fmt.Errorf("volume must be greater than zero and at most 100000 ml")
	}
	if s.AliquotCount < 0 || s.AliquotCount > 10000 {
		return fmt.Errorf("aliquot count must be between zero and 10000")
	}
	if s.State == constants.SpecimenStateAliquoted && s.AliquotCount < 1 {
		return fmt.Errorf("aliquoted specimens must have at least one aliquot")
	}
	if s.State == constants.SpecimenStateStored {
		if s.StorageContainerID == nil || *s.StorageContainerID == 0 || s.Position == "" {
			return fmt.Errorf("stored specimens require a container and position")
		}
	}
	if length := len([]rune(s.CurrentCustodian)); length < 2 || length > 100 {
		return fmt.Errorf("current custodian must contain 2-100 characters")
	}
	if s.ReceivedAt.IsZero() {
		return fmt.Errorf("received time is required")
	}
	if s.ExpiresAt != nil && !s.ExpiresAt.After(s.ReceivedAt) {
		return fmt.Errorf("expiry time must be after received time")
	}
	if len([]rune(s.Position)) > 120 {
		return fmt.Errorf("position cannot exceed 120 characters")
	}
	if len([]rune(s.Notes)) > 1000 {
		return fmt.Errorf("notes cannot exceed 1000 characters")
	}
	return nil
}

func (s Specimen) LocationLabel() string {
	if s.StorageContainer != nil {
		parts := []string{s.StorageContainer.Location, s.StorageContainer.Code, s.Position}
		return strings.Trim(strings.Join(parts, " / "), " / ")
	}
	if s.Position != "" {
		return s.Position
	}
	if s.State == constants.SpecimenStateReleased {
		return "released"
	}
	if s.State == constants.SpecimenStateDisposed {
		return "disposed"
	}
	return "intake"
}

func (s Specimen) Expired(at time.Time) bool {
	return s.ExpiresAt != nil && !s.ExpiresAt.After(at)
}

func (s Specimen) Mutable() bool {
	return s.State != constants.SpecimenStateDisposed
}

func (s Specimen) HasPreparedTransfer() bool {
	for _, transfer := range s.Transfers {
		if transfer.State == constants.TransferStatePrepared {
			return true
		}
	}
	return false
}
