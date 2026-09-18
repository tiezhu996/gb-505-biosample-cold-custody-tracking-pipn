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
	Reservation    *SlotReservation        `gorm:"foreignKey:TransferID" json:"reservation,omitempty"`
	// ReservationState 与 ConflictReason 由服务层按当前时间计算，仅用于展示，不落库。
	ReservationState string `gorm:"-" json:"reservationState,omitempty"`
	ConflictReason   string `gorm:"-" json:"conflictReason,omitempty"`
}

// SlotReservation 记录交接单对目标格位的预约：接收成功后转为实际占用，
// 拒绝、取消或到期则释放，样本原位置不变。
type SlotReservation struct {
	Base
	TransferID    uint                       `gorm:"uniqueIndex;not null" json:"transferId"`
	Transfer      *CustodyTransfer           `json:"-"`
	ContainerID   uint                       `gorm:"index;not null" json:"containerId"`
	Container     *StorageContainer          `json:"container,omitempty"`
	Position      string                     `gorm:"size:120;not null" json:"position"`
	State         constants.ReservationState `gorm:"size:20;index;not null;default:'active'" json:"state"`
	ExpiresAt     time.Time                  `gorm:"index;not null" json:"expiresAt"`
	ReleasedAt    *time.Time                 `json:"releasedAt,omitempty"`
	ReleaseReason string                     `gorm:"size:200" json:"releaseReason,omitempty"`
}

func (r *SlotReservation) Normalize() {
	r.Position = strings.TrimSpace(r.Position)
	r.ReleaseReason = strings.TrimSpace(r.ReleaseReason)
	if r.State == "" {
		r.State = constants.ReservationActive
	}
}

func (r SlotReservation) Validate() error {
	if r.ContainerID == 0 {
		return fmt.Errorf("reservation requires a container")
	}
	if length := len([]rune(r.Position)); length < 1 || length > 120 {
		return fmt.Errorf("reservation position must contain 1-120 characters")
	}
	if !r.State.Valid() {
		return fmt.Errorf("unsupported reservation state: %s", r.State)
	}
	if r.ExpiresAt.IsZero() {
		return fmt.Errorf("reservation expiry time is required")
	}
	if r.State != constants.ReservationActive && r.ReleasedAt == nil {
		return fmt.Errorf("released reservation requires a release time")
	}
	if len([]rune(r.ReleaseReason)) > 200 {
		return fmt.Errorf("reservation release reason is too long")
	}
	return nil
}

// EffectiveState 按当前时间折算预约状态：到期未处理的预约视为已过期。
func (r SlotReservation) EffectiveState(at time.Time) constants.ReservationState {
	if r.State == constants.ReservationActive && !r.ExpiresAt.After(at) {
		return constants.ReservationExpired
	}
	return r.State
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
	if (t.ToContainerID == nil) != (t.ToPosition == "") {
		return fmt.Errorf("target container and position must be provided together")
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
