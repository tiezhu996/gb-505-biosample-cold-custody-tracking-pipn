package dto

import "time"

type PageQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
}

func (q PageQuery) Normalize() PageQuery {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 10
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	return q
}

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

type TransitionRequest struct {
	State  string `json:"state" binding:"required"`
	Reason string `json:"reason" binding:"max=1000"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type CustodyOverview struct {
	GeneratedAt        time.Time     `json:"generatedAt"`
	TotalSpecimens     int64         `json:"totalSpecimens"`
	SpecimenStates     []StatusCount `json:"specimenStates"`
	ActiveContainers   int64         `json:"activeContainers"`
	ContainersAtRisk   int64         `json:"containersAtRisk"`
	PreparedTransfers  int64         `json:"preparedTransfers"`
	AcceptedToday      int64         `json:"acceptedToday"`
	PendingReviews     int64         `json:"pendingReviews"`
	AuditEventsToday   int64         `json:"auditEventsToday"`
	ExpiredSpecimens   int64         `json:"expiredSpecimens"`
	CapacityUsed       int64         `json:"capacityUsed"`
	CapacityTotal      int64         `json:"capacityTotal"`
	CapacityPercentage float64       `json:"capacityPercentage"`
}

func (o CustodyOverview) StorageSignal() string {
	if o.ContainersAtRisk > 0 || o.ExpiredSpecimens > 0 {
		return "attention"
	}
	if o.PreparedTransfers > 0 || o.PendingReviews > 0 {
		return "monitor"
	}
	return "stable"
}
