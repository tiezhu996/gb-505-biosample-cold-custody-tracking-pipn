package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
)

type StorageFilter struct {
	dto.PageQuery
	TemperatureZone string `form:"temperatureZone"`
	Status          string `form:"status"`
}

type StorageRepository interface {
	List(context.Context, StorageFilter) ([]model.StorageContainer, int64, error)
	Find(context.Context, uint) (*model.StorageContainer, error)
	FindForUpdate(context.Context, *gorm.DB, uint) (*model.StorageContainer, error)
	FindByCode(context.Context, string) (*model.StorageContainer, error)
	Create(context.Context, *model.StorageContainer) error
	Save(context.Context, *model.StorageContainer) error
	CountStoredSpecimens(context.Context, uint) (int64, error)
}

type storageRepository struct{ db *gorm.DB }

func NewStorageRepository(db *gorm.DB) StorageRepository {
	return &storageRepository{db: db}
}

func (r *storageRepository) List(ctx context.Context, filter StorageFilter) ([]model.StorageContainer, int64, error) {
	query := filter.PageQuery.Normalize()
	db := r.db.WithContext(ctx).Model(&model.StorageContainer{})
	if zone := strings.TrimSpace(filter.TemperatureZone); zone != "" {
		db = db.Where("temperature_zone = ?", zone)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		db = db.Where("code ILIKE ? OR name ILIKE ? OR container_type ILIKE ? OR location ILIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.StorageContainer, 0)
	err := db.Preload("Specimens", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("updated_at DESC").Limit(20)
	}).Order("active DESC, status ASC, code ASC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (r *storageRepository) Find(ctx context.Context, id uint) (*model.StorageContainer, error) {
	var item model.StorageContainer
	err := r.db.WithContext(ctx).Preload("Specimens", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("position ASC")
	}).First(&item, id).Error
	return &item, err
}

func (r *storageRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, id uint) (*model.StorageContainer, error) {
	var item model.StorageContainer
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error
	return &item, err
}

func (r *storageRepository) FindByCode(ctx context.Context, code string) (*model.StorageContainer, error) {
	var item model.StorageContainer
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	return &item, err
}

func (r *storageRepository) Create(ctx context.Context, item *model.StorageContainer) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *storageRepository) Save(ctx context.Context, item *model.StorageContainer) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *storageRepository) CountStoredSpecimens(ctx context.Context, containerID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Specimen{}).
		Where("storage_container_id = ? AND state NOT IN ?", containerID, []string{"released", "disposed"}).
		Count(&count).Error
	return count, err
}
