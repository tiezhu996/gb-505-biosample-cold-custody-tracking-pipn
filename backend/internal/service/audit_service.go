package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
	"biosample-cold-custody-tracking/backend/internal/repository"
)

type Actor struct {
	ID        uint
	Name      string
	RequestID string
	IP        string
}

type AuditService interface {
	Record(context.Context, Actor, string, string, uint, any, any) error
	List(context.Context, repository.AuditFilter) (dto.PageResult[model.AuditLog], error)
	Verify(context.Context) error
}

type auditService struct{ repo repository.AuditRepository }

func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) Record(ctx context.Context, actor Actor, action, entityType string, entityID uint, before, after any) error {
	if actor.ID == 0 || strings.TrimSpace(actor.Name) == "" {
		return fmt.Errorf("audit actor is required")
	}
	if strings.TrimSpace(actor.RequestID) == "" {
		return fmt.Errorf("audit request ID is required")
	}
	beforeJSON, err := auditJSON(before)
	if err != nil {
		return fmt.Errorf("serialize audit before state: %w", err)
	}
	afterJSON, err := auditJSON(after)
	if err != nil {
		return fmt.Errorf("serialize audit after state: %w", err)
	}
	beforeLocation, beforeCustodian := custodyCoordinates(before)
	afterLocation, afterCustodian := custodyCoordinates(after)
	entry := &model.AuditLog{
		RequestID:       actor.RequestID,
		ActorID:         actor.ID,
		ActorName:       strings.TrimSpace(actor.Name),
		Action:          strings.TrimSpace(action),
		EntityType:      strings.TrimSpace(entityType),
		EntityID:        entityID,
		BeforeState:     beforeJSON,
		AfterState:      afterJSON,
		BeforeLocation:  beforeLocation,
		AfterLocation:   afterLocation,
		BeforeCustodian: beforeCustodian,
		AfterCustodian:  afterCustodian,
		IPAddress:       strings.TrimSpace(actor.IP),
	}
	return s.repo.Append(ctx, entry)
}

func (s *auditService) List(ctx context.Context, filter repository.AuditFilter) (dto.PageResult[model.AuditLog], error) {
	query := filter.PageQuery.Normalize()
	filter.PageQuery = query
	items, total, err := s.repo.List(ctx, filter)
	return dto.PageResult[model.AuditLog]{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

func (s *auditService) Verify(ctx context.Context) error {
	return s.repo.VerifyChain(ctx)
}

func auditJSON(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func custodyCoordinates(value any) (string, string) {
	switch item := value.(type) {
	case model.Specimen:
		return item.LocationLabel(), item.CurrentCustodian
	case *model.Specimen:
		if item != nil {
			return item.LocationLabel(), item.CurrentCustodian
		}
	case model.CustodyTransfer:
		if item.MovesSpecimen() {
			return item.ToLocation, item.ToCustodian
		}
		return item.FromLocation, item.FromCustodian
	case *model.CustodyTransfer:
		if item != nil {
			return custodyCoordinates(*item)
		}
	case model.StorageContainer:
		return strings.Trim(strings.Join([]string{item.Location, item.Code}, " / "), " / "), ""
	case *model.StorageContainer:
		if item != nil {
			return custodyCoordinates(*item)
		}
	}
	return "", ""
}
