package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
)

var (
	ErrTransferAlreadyResolved = errors.New("custody transfer is already resolved")
	ErrSpecimenCustodyChanged  = errors.New("specimen custody changed after transfer preparation")
	ErrTargetContainerFull     = errors.New("target storage container is not available or is full")
	ErrPositionOccupied        = errors.New("target storage position is already occupied")
	ErrTemperatureExcursion    = errors.New("recorded temperature is outside the target container range")
)

type TransferFilter struct {
	dto.PageQuery
	State      string `form:"state"`
	SpecimenID uint   `form:"specimenId"`
}

type TransferResolution struct {
	State          constants.TransferState
	ToContainerID  *uint
	ToPosition     string
	TemperatureC   *float64
	Reason         string
	ResolvedByID   uint
	ResolvedByName string
	ResolvedAt     time.Time
}

type TransferRepository interface {
	List(context.Context, TransferFilter) ([]model.CustodyTransfer, int64, error)
	Find(context.Context, uint) (*model.CustodyTransfer, error)
	FindByNumber(context.Context, string) (*model.CustodyTransfer, error)
	Create(context.Context, *model.CustodyTransfer) error
	CountPreparedForSpecimen(context.Context, uint) (int64, error)
	Resolve(context.Context, uint, TransferResolution) (*model.CustodyTransfer, *model.Specimen, model.Specimen, error)
}

type transferRepository struct{ db *gorm.DB }

func NewTransferRepository(db *gorm.DB) TransferRepository {
	return &transferRepository{db: db}
}

func (r *transferRepository) List(ctx context.Context, filter TransferFilter) ([]model.CustodyTransfer, int64, error) {
	query := filter.PageQuery.Normalize()
	db := r.db.WithContext(ctx).Model(&model.CustodyTransfer{})
	if state := strings.TrimSpace(filter.State); state != "" {
		db = db.Where("state = ?", state)
	}
	if filter.SpecimenID > 0 {
		db = db.Where("specimen_id = ?", filter.SpecimenID)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		db = db.Where("transfer_no ILIKE ? OR from_custodian ILIKE ? OR to_custodian ILIKE ? OR from_location ILIKE ? OR to_location ILIKE ?", like, like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.CustodyTransfer, 0)
	err := db.Preload("Specimen").Preload("Specimen.StorageContainer").Preload("ToContainer").
		Order("prepared_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (r *transferRepository) Find(ctx context.Context, id uint) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Preload("Specimen").Preload("Specimen.StorageContainer").Preload("ToContainer").First(&item, id).Error
	return &item, err
}

func (r *transferRepository) FindByNumber(ctx context.Context, number string) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Where("transfer_no = ?", number).First(&item).Error
	return &item, err
}

func (r *transferRepository) Create(ctx context.Context, item *model.CustodyTransfer) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *transferRepository) CountPreparedForSpecimen(ctx context.Context, specimenID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.CustodyTransfer{}).
		Where("specimen_id = ? AND state = ?", specimenID, constants.TransferStatePrepared).Count(&count).Error
	return count, err
}

func (r *transferRepository) Resolve(ctx context.Context, transferID uint, resolution TransferResolution) (*model.CustodyTransfer, *model.Specimen, model.Specimen, error) {
	var transfer model.CustodyTransfer
	var specimen model.Specimen
	var before model.Specimen
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&transfer, transferID).Error; err != nil {
			return err
		}
		if transfer.State != constants.TransferStatePrepared {
			return ErrTransferAlreadyResolved
		}
		if err := tx.Preload("StorageContainer").Clauses(clause.Locking{Strength: "UPDATE"}).First(&specimen, transfer.SpecimenID).Error; err != nil {
			return err
		}
		before = specimen
		if specimen.CurrentCustodian != transfer.FromCustodian || specimen.LocationLabel() != transfer.FromLocation {
			return ErrSpecimenCustodyChanged
		}

		transfer.State = resolution.State
		transfer.ToContainerID = resolution.ToContainerID
		transfer.ToPosition = strings.TrimSpace(resolution.ToPosition)
		transfer.TemperatureC = resolution.TemperatureC
		transfer.Reason = strings.TrimSpace(resolution.Reason)
		transfer.AcceptedByID = &resolution.ResolvedByID
		transfer.AcceptedByName = strings.TrimSpace(resolution.ResolvedByName)
		transfer.ResolvedAt = &resolution.ResolvedAt
		transfer.Normalize()

		if transfer.State == constants.TransferStateAccepted {
			if transfer.ToContainerID == nil || *transfer.ToContainerID == 0 {
				return ErrTargetContainerFull
			}
			var target model.StorageContainer
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, *transfer.ToContainerID).Error; err != nil {
				return err
			}
			if !target.CanReceive() && (specimen.StorageContainerID == nil || *specimen.StorageContainerID != target.ID) {
				return ErrTargetContainerFull
			}
			if transfer.TemperatureC != nil && !target.AcceptsTemperature(*transfer.TemperatureC) {
				return ErrTemperatureExcursion
			}
			var occupied int64
			positionQuery := tx.Model(&model.Specimen{}).
				Where("storage_container_id = ? AND position = ? AND id <> ? AND state NOT IN ?", target.ID, transfer.ToPosition, specimen.ID, []constants.SpecimenState{constants.SpecimenStateReleased, constants.SpecimenStateDisposed})
			if err := positionQuery.Count(&occupied).Error; err != nil {
				return err
			}
			if occupied > 0 {
				return ErrPositionOccupied
			}
			oldContainerID := specimen.StorageContainerID
			if oldContainerID == nil || *oldContainerID != target.ID {
				if oldContainerID != nil && *oldContainerID > 0 {
					if err := tx.Model(&model.StorageContainer{}).Where("id = ?", *oldContainerID).
						UpdateColumn("occupied", gorm.Expr("GREATEST(occupied - 1, 0)")).Error; err != nil {
						return err
					}
				}
				result := tx.Model(&model.StorageContainer{}).
					Where("id = ? AND active = ? AND status = ? AND occupied < capacity", target.ID, true, "available").
					UpdateColumn("occupied", gorm.Expr("occupied + 1"))
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrTargetContainerFull
				}
			}
			specimen.StorageContainerID = &target.ID
			specimen.StorageContainer = &target
			specimen.Position = transfer.ToPosition
			specimen.CurrentCustodian = transfer.ToCustodian
			if specimen.State == constants.SpecimenStateReceived || specimen.State == constants.SpecimenStateAliquoted {
				specimen.State = constants.SpecimenStateStored
			}
			if err := specimen.Validate(); err != nil {
				return fmt.Errorf("validate moved specimen: %w", err)
			}
			if err := tx.Save(&specimen).Error; err != nil {
				return err
			}
		}
		if err := transfer.Validate(); err != nil {
			return fmt.Errorf("validate resolved transfer: %w", err)
		}
		return tx.Save(&transfer).Error
	})
	if err != nil {
		return nil, nil, model.Specimen{}, err
	}
	resolved, err := r.Find(ctx, transfer.ID)
	if err != nil {
		return nil, nil, model.Specimen{}, err
	}
	return resolved, &specimen, before, nil
}
