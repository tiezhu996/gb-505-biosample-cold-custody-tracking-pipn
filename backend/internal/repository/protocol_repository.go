package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
)

var ErrSpecimenNotReviewable = errors.New("specimen is not in a reviewable state")

type ProtocolFilter struct {
	dto.PageQuery
	Decision   string `form:"decision"`
	SpecimenID uint   `form:"specimenId"`
}

type ProtocolRepository interface {
	List(context.Context, ProtocolFilter) ([]model.ProtocolReview, int64, error)
	Find(context.Context, uint) (*model.ProtocolReview, error)
	LatestForSpecimen(context.Context, uint) (*model.ProtocolReview, error)
	Create(context.Context, *model.ProtocolReview) (*model.ProtocolReview, model.Specimen, model.Specimen, error)
}

type protocolRepository struct{ db *gorm.DB }

func NewProtocolRepository(db *gorm.DB) ProtocolRepository {
	return &protocolRepository{db: db}
}

func (r *protocolRepository) List(ctx context.Context, filter ProtocolFilter) ([]model.ProtocolReview, int64, error) {
	query := filter.PageQuery.Normalize()
	db := r.db.WithContext(ctx).Model(&model.ProtocolReview{})
	if decision := strings.TrimSpace(filter.Decision); decision != "" {
		db = db.Where("decision = ?", decision)
	}
	if filter.SpecimenID > 0 {
		db = db.Where("specimen_id = ?", filter.SpecimenID)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		db = db.Where("protocol_code ILIKE ? OR reviewer_name ILIKE ? OR notes ILIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.ProtocolReview, 0)
	err := db.Preload("Specimen").Order("reviewed_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (r *protocolRepository) Find(ctx context.Context, id uint) (*model.ProtocolReview, error) {
	var item model.ProtocolReview
	err := r.db.WithContext(ctx).Preload("Specimen").First(&item, id).Error
	return &item, err
}

func (r *protocolRepository) LatestForSpecimen(ctx context.Context, specimenID uint) (*model.ProtocolReview, error) {
	var item model.ProtocolReview
	err := r.db.WithContext(ctx).Where("specimen_id = ?", specimenID).Order("reviewed_at DESC, id DESC").First(&item).Error
	return &item, err
}

func (r *protocolRepository) Create(ctx context.Context, review *model.ProtocolReview) (*model.ProtocolReview, model.Specimen, model.Specimen, error) {
	var specimen model.Specimen
	var before model.Specimen
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&specimen, review.SpecimenID).Error; err != nil {
			return err
		}
		before = specimen
		if specimen.ProtocolCode != review.ProtocolCode || specimen.State == constants.SpecimenStateDisposed {
			return ErrSpecimenNotReviewable
		}
		if review.Decision == constants.DecisionApproved {
			if specimen.State != constants.SpecimenStateStored {
				return ErrSpecimenNotReviewable
			}
			if specimen.StorageContainerID != nil {
				if err := tx.Model(&model.StorageContainer{}).Where("id = ?", *specimen.StorageContainerID).
					UpdateColumn("occupied", gorm.Expr("GREATEST(occupied - 1, 0)")).Error; err != nil {
					return err
				}
			}
			specimen.State = constants.SpecimenStateReleased
			specimen.StorageContainerID = nil
			specimen.Position = ""
			if err := tx.Save(&specimen).Error; err != nil {
				return err
			}
		}
		return tx.Create(review).Error
	})
	if err != nil {
		return nil, model.Specimen{}, model.Specimen{}, err
	}
	created, err := r.Find(ctx, review.ID)
	return created, specimen, before, err
}
