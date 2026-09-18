package dto

import (
	"time"

	"biosample-cold-custody-tracking/backend/internal/constants"
)

type CreateTransferRequest struct {
	SpecimenID    uint     `json:"specimenId" binding:"required"`
	TransferNo    string   `json:"transferNo" binding:"required,min=3,max=50"`
	FromCustodian string   `json:"fromCustodian" binding:"required,min=2,max=100"`
	ToCustodian   string   `json:"toCustodian" binding:"required,min=2,max=100"`
	FromLocation  string   `json:"fromLocation" binding:"required,min=2,max=200"`
	ToLocation    string   `json:"toLocation" binding:"required,min=2,max=200"`
	TemperatureC  *float64 `json:"temperatureC" binding:"omitempty,gte=-210,lte=40"`
	Reason        string   `json:"reason" binding:"omitempty,max=1000"`
}

type ResolveTransferRequest struct {
	State         constants.TransferState `json:"state" binding:"required,oneof=accepted rejected cancelled"`
	ToContainerID *uint                   `json:"toContainerId"`
	ToPosition    *string                 `json:"toPosition" binding:"omitempty,max=120"`
	TemperatureC  *float64                `json:"temperatureC" binding:"omitempty,gte=-210,lte=40"`
	Reason        string                  `json:"reason" binding:"omitempty,max=1000"`
}

type CreateProtocolReviewRequest struct {
	SpecimenID        uint                     `json:"specimenId" binding:"required"`
	ProtocolCode      string                   `json:"protocolCode" binding:"required,min=3,max=80"`
	Decision          constants.ReviewDecision `json:"decision" binding:"required,oneof=approved hold rejected"`
	ConsentVerified   bool                     `json:"consentVerified"`
	ScopeVerified     bool                     `json:"scopeVerified"`
	RetentionUntil    *time.Time               `json:"retentionUntil"`
	DocumentObjectKey *string                  `json:"documentObjectKey" binding:"omitempty,max=512"`
	Notes             string                   `json:"notes" binding:"omitempty,max=2000"`
}
