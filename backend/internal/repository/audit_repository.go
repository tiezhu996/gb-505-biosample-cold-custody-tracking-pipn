package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
)

const auditChainLockID int64 = 50520260822

type AuditFilter struct {
	dto.PageQuery
	EntityType string `form:"entityType"`
	ActorID    uint   `form:"actorId"`
	RequestID  string `form:"requestId"`
}

type AuditRepository interface {
	Append(context.Context, *model.AuditLog) error
	List(context.Context, AuditFilter) ([]model.AuditLog, int64, error)
	FindByRequestID(context.Context, string) ([]model.AuditLog, error)
	VerifyChain(context.Context) error
}

type auditRepository struct{ db *gorm.DB }

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Append(ctx context.Context, entry *model.AuditLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", auditChainLockID).Error; err != nil {
			return fmt.Errorf("lock audit chain: %w", err)
		}
		var previous model.AuditLog
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id DESC").First(&previous).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("read audit chain head: %w", err)
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now().UTC()
		}
		previousHash := ""
		if err == nil {
			previousHash = previous.EntryHash
		}
		entry.Seal(previousHash)
		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("append audit entry: %w", err)
		}
		return nil
	})
}

func (r *auditRepository) List(ctx context.Context, filter AuditFilter) ([]model.AuditLog, int64, error) {
	query := filter.PageQuery.Normalize()
	db := r.db.WithContext(ctx).Model(&model.AuditLog{})
	if value := strings.TrimSpace(filter.EntityType); value != "" {
		db = db.Where("entity_type = ?", value)
	}
	if filter.ActorID > 0 {
		db = db.Where("actor_id = ?", filter.ActorID)
	}
	if value := strings.TrimSpace(filter.RequestID); value != "" {
		db = db.Where("request_id = ?", value)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		db = db.Where("action ILIKE ? OR actor_name ILIKE ? OR request_id ILIKE ? OR entity_type ILIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	entries := make([]model.AuditLog, 0)
	err := db.Order("id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&entries).Error
	return entries, total, err
}

func (r *auditRepository) FindByRequestID(ctx context.Context, requestID string) ([]model.AuditLog, error) {
	entries := make([]model.AuditLog, 0)
	err := r.db.WithContext(ctx).Where("request_id = ?", requestID).Order("id ASC").Find(&entries).Error
	return entries, err
}

func (r *auditRepository) VerifyChain(ctx context.Context) error {
	entries := make([]model.AuditLog, 0)
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&entries).Error; err != nil {
		return err
	}
	previousHash := ""
	for _, entry := range entries {
		if entry.PreviousHash != previousHash || !entry.IntegrityValid() {
			return fmt.Errorf("audit chain integrity failure at entry %d", entry.ID)
		}
		previousHash = entry.EntryHash
	}
	return nil
}
