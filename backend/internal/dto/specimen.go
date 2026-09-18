package dto

import "time"

type CreateSpecimenRequest struct {
	AccessionNo      string     `json:"accessionNo" binding:"required,min=3,max=50"`
	SampleType       string     `json:"sampleType" binding:"required,min=2,max=100"`
	SubjectCode      string     `json:"subjectCode" binding:"required,min=3,max=80"`
	ProtocolCode     string     `json:"protocolCode" binding:"required,min=3,max=80"`
	VolumeML         float64    `json:"volumeMl" binding:"required,gt=0,lte=100000"`
	AliquotCount     int        `json:"aliquotCount" binding:"omitempty,min=0,max=10000"`
	CurrentCustodian string     `json:"currentCustodian" binding:"required,min=2,max=100"`
	ReceivedAt       *time.Time `json:"receivedAt"`
	ExpiresAt        *time.Time `json:"expiresAt"`
	Notes            string     `json:"notes" binding:"max=1000"`
}

type UpdateSpecimenRequest struct {
	SampleType       *string    `json:"sampleType" binding:"omitempty,min=2,max=100"`
	ProtocolCode     *string    `json:"protocolCode" binding:"omitempty,min=3,max=80"`
	VolumeML         *float64   `json:"volumeMl" binding:"omitempty,gt=0,lte=100000"`
	AliquotCount     *int       `json:"aliquotCount" binding:"omitempty,min=0,max=10000"`
	CurrentCustodian *string    `json:"currentCustodian" binding:"omitempty,min=2,max=100"`
	ExpiresAt        *time.Time `json:"expiresAt"`
	Notes            *string    `json:"notes" binding:"omitempty,max=1000"`
}
