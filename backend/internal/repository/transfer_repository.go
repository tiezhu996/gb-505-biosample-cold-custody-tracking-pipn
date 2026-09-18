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
	ErrTargetContainerMissing  = errors.New("target storage container does not exist")
	ErrPositionOccupied        = errors.New("target storage position is already occupied")
	ErrPositionReserved        = errors.New("target storage position is reserved by another custody transfer")
	ErrReservationExpired      = errors.New("target position reservation has expired")
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
	applyReservationView(items)
	return items, total, err
}

func (r *transferRepository) Find(ctx context.Context, id uint) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Preload("Specimen").Preload("Specimen.StorageContainer").Preload("ToContainer").First(&item, id).Error
	applyReservationView([]model.CustodyTransfer{item})
	return &item, err
}

func (r *transferRepository) FindByNumber(ctx context.Context, number string) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Where("transfer_no = ?", number).First(&item).Error
	return &item, err
}

func (r *transferRepository) Create(ctx context.Context, item *model.CustodyTransfer) error {
	if !item.HasReservation() {
		return r.db.WithContext(ctx).Create(item).Error
	}
	// 预约创建与冲突校验在目标容器行锁内串行执行，保证同一格位只有一份有效预约。
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var target model.StorageContainer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, *item.ToContainerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTargetContainerMissing
			}
			return err
		}
		if !target.Active || target.Status != "available" {
			return ErrTargetContainerFull
		}
		if item.TemperatureC != nil && !target.AcceptsTemperature(*item.TemperatureC) {
			return ErrTemperatureExcursion
		}
		var specimen model.Specimen
		if err := tx.Select("id", "storage_container_id").First(&specimen, item.SpecimenID).Error; err != nil {
			return err
		}
		if specimen.StorageContainerID == nil || *specimen.StorageContainerID != target.ID {
			reserved, err := countActiveReservations(tx, target.ID, "", now, 0)
			if err != nil {
				return err
			}
			if target.Occupied+int(reserved) >= target.Capacity {
				return ErrTargetContainerFull
			}
		}
		var occupied int64
		if err := tx.Model(&model.Specimen{}).
			Where("storage_container_id = ? AND position = ? AND id <> ? AND state NOT IN ?", target.ID, item.ToPosition, item.SpecimenID, []constants.SpecimenState{constants.SpecimenStateReleased, constants.SpecimenStateDisposed}).
			Count(&occupied).Error; err != nil {
			return err
		}
		if occupied > 0 {
			return ErrPositionOccupied
		}
		conflicting, err := countActiveReservations(tx, target.ID, item.ToPosition, now, 0)
		if err != nil {
			return err
		}
		if conflicting > 0 {
			return ErrPositionReserved
		}
		return tx.Create(item).Error
	})
}

// countActiveReservations 统计容器内仍未到期的格位预约；position 为空时统计整个容器。
func countActiveReservations(tx *gorm.DB, containerID uint, position string, now time.Time, excludeID uint) (int64, error) {
	query := tx.Model(&model.CustodyTransfer{}).
		Where("to_container_id = ? AND state = ? AND reservation_status = ? AND reservation_expires_at > ?",
			containerID, constants.TransferStatePrepared, constants.ReservationActive, now)
	if position != "" {
		query = query.Where("to_position = ?", position)
	}
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
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

		now := resolution.ResolvedAt
		transfer.State = resolution.State
		transfer.TemperatureC = resolution.TemperatureC
		transfer.Reason = strings.TrimSpace(resolution.Reason)
		transfer.AcceptedByID = &resolution.ResolvedByID
		transfer.AcceptedByName = strings.TrimSpace(resolution.ResolvedByName)
		transfer.ResolvedAt = &resolution.ResolvedAt

		if transfer.State == constants.TransferStateAccepted {
			if transfer.HasReservation() {
				// 已预约格位以预约记录为准；到期预约已释放，受理直接失败且记录不变。
				if transfer.ReservationEffective(now) != constants.ReservationActive {
					return ErrReservationExpired
				}
			} else {
				transfer.ToContainerID = resolution.ToContainerID
				transfer.ToPosition = strings.TrimSpace(resolution.ToPosition)
			}
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
			reserved, err := countActiveReservations(tx, target.ID, transfer.ToPosition, now, transfer.ID)
			if err != nil {
				return err
			}
			if reserved > 0 {
				return ErrPositionReserved
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
			if transfer.HasReservation() {
				transfer.ReservationStatus = constants.ReservationConsumed
				transfer.ReservationNote = "接收成功，预约转为实际占用"
			}
		} else {
			if transfer.HasReservation() {
				// 释放预约但保留目标容器与格位快照，便于审计追溯和交接页展示。
				transfer.ReservationStatus = constants.ReservationReleased
				if transfer.State == constants.TransferStateRejected {
					transfer.ReservationNote = "交接已拒绝，预约已释放"
				} else {
					transfer.ReservationNote = "交接已取消，预约已释放"
				}
			} else {
				transfer.ToContainerID = nil
				transfer.ToPosition = ""
			}
		}
		transfer.Normalize()
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

// applyReservationView 把存储中的预约状态换算为考虑到期时间的展示状态。
func applyReservationView(items []model.CustodyTransfer) {
	now := time.Now().UTC()
	for i := range items {
		items[i].ReservationStatus = items[i].ReservationEffective(now)
	}
}
