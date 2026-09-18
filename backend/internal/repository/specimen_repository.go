package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
)

type SpecimenFilter struct {
	dto.PageQuery
	State              string `form:"state"`
	StorageContainerID uint   `form:"storageContainerId"`
	ProtocolCode       string `form:"protocolCode"`
}

type SpecimenRepository interface {
	List(context.Context, SpecimenFilter) ([]model.Specimen, int64, error)
	Find(context.Context, uint) (*model.Specimen, error)
	FindForUpdate(context.Context, *gorm.DB, uint) (*model.Specimen, error)
	FindByAccession(context.Context, string) (*model.Specimen, error)
	Create(context.Context, *model.Specimen) error
	Save(context.Context, *model.Specimen) error
	Transaction(context.Context, func(*gorm.DB) error) error
	Overview(context.Context) (*dto.CustodyOverview, error)
}

type specimenRepository struct{ db *gorm.DB }

func NewSpecimenRepository(db *gorm.DB) SpecimenRepository {
	return &specimenRepository{db: db}
}

func (r *specimenRepository) List(ctx context.Context, filter SpecimenFilter) ([]model.Specimen, int64, error) {
	query := filter.PageQuery.Normalize()
	db := r.db.WithContext(ctx).Model(&model.Specimen{})
	if state := strings.TrimSpace(filter.State); state != "" {
		db = db.Where("state = ?", state)
	}
	if filter.StorageContainerID > 0 {
		db = db.Where("storage_container_id = ?", filter.StorageContainerID)
	}
	if protocol := strings.TrimSpace(filter.ProtocolCode); protocol != "" {
		db = db.Where("protocol_code = ?", strings.ToUpper(protocol))
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		db = db.Where("accession_no ILIKE ? OR sample_type ILIKE ? OR subject_code ILIKE ? OR protocol_code ILIKE ? OR current_custodian ILIKE ?", like, like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.Specimen, 0)
	err := db.Preload("StorageContainer").Preload("Transfers", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("prepared_at DESC").Limit(10)
	}).Preload("ProtocolReviews", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("reviewed_at DESC").Limit(10)
	}).Order("received_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (r *specimenRepository) Find(ctx context.Context, id uint) (*model.Specimen, error) {
	var item model.Specimen
	err := r.db.WithContext(ctx).
		Preload("StorageContainer").
		Preload("Transfers", func(tx *gorm.DB) *gorm.DB { return tx.Order("prepared_at DESC") }).
		Preload("Transfers.ToContainer").
		Preload("ProtocolReviews", func(tx *gorm.DB) *gorm.DB { return tx.Order("reviewed_at DESC") }).
		First(&item, id).Error
	return &item, err
}

func (r *specimenRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, id uint) (*model.Specimen, error) {
	var item model.Specimen
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error
	return &item, err
}

func (r *specimenRepository) FindByAccession(ctx context.Context, accessionNo string) (*model.Specimen, error) {
	var item model.Specimen
	err := r.db.WithContext(ctx).Where("accession_no = ?", accessionNo).First(&item).Error
	return &item, err
}

func (r *specimenRepository) Create(ctx context.Context, item *model.Specimen) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *specimenRepository) Save(ctx context.Context, item *model.Specimen) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *specimenRepository) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func (r *specimenRepository) Overview(ctx context.Context) (*dto.CustodyOverview, error) {
	db := r.db.WithContext(ctx)
	now := time.Now()
	result := &dto.CustodyOverview{GeneratedAt: now, SpecimenStates: make([]dto.StatusCount, 0)}
	if err := db.Model(&model.Specimen{}).Count(&result.TotalSpecimens).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.Specimen{}).Select("state AS status, count(*) AS count").Group("state").Order("state").Scan(&result.SpecimenStates).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.StorageContainer{}).Where("active = ?", true).Count(&result.ActiveContainers).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.StorageContainer{}).Where("status IN ?", []string{"maintenance", "alarm"}).Count(&result.ContainersAtRisk).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.CustodyTransfer{}).Where("state = ?", constants.TransferStatePrepared).Count(&result.PreparedTransfers).Error; err != nil {
		return nil, err
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	db.Model(&model.CustodyTransfer{}).Where("state = ? AND resolved_at >= ?", constants.TransferStateAccepted, dayStart).Count(&result.AcceptedToday)
	db.Model(&model.Specimen{}).Where("expires_at < ? AND state NOT IN ?", now, []constants.SpecimenState{constants.SpecimenStateReleased, constants.SpecimenStateDisposed}).Count(&result.ExpiredSpecimens)
	db.Model(&model.AuditLog{}).Where("created_at >= ?", dayStart).Count(&result.AuditEventsToday)
	db.Model(&model.StorageContainer{}).Select("COALESCE(SUM(occupied), 0)").Scan(&result.CapacityUsed)
	db.Model(&model.StorageContainer{}).Select("COALESCE(SUM(capacity), 0)").Scan(&result.CapacityTotal)
	if result.CapacityTotal > 0 {
		result.CapacityPercentage = float64(result.CapacityUsed) / float64(result.CapacityTotal) * 100
	}
	var reviewedIDs []uint
	db.Model(&model.ProtocolReview{}).Select("DISTINCT specimen_id").Where("decision = ?", constants.DecisionApproved).Scan(&reviewedIDs)
	pending := db.Model(&model.Specimen{}).Where("state = ?", constants.SpecimenStateStored)
	if len(reviewedIDs) > 0 {
		pending = pending.Where("id NOT IN ?", reviewedIDs)
	}
	pending.Count(&result.PendingReviews)
	return result, nil
}
