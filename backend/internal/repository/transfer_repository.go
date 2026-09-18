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
	ErrSlotReservedByOther     = errors.New("target storage position is reserved by another transfer")
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
	CreateWithReservation(context.Context, *model.CustodyTransfer, *model.SlotReservation, *model.Specimen) error
	CountPreparedForSpecimen(context.Context, uint) (int64, error)
	Resolve(context.Context, uint, TransferResolution) (*model.CustodyTransfer, *model.Specimen, model.Specimen, error)
	SlotConflicts(context.Context, []model.CustodyTransfer) (map[uint]string, error)
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
	err := db.Preload("Specimen").Preload("Specimen.StorageContainer").Preload("ToContainer").Preload("Reservation").
		Order("prepared_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (r *transferRepository) Find(ctx context.Context, id uint) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Preload("Specimen").Preload("Specimen.StorageContainer").Preload("ToContainer").Preload("Reservation").First(&item, id).Error
	return &item, err
}

func (r *transferRepository) FindByNumber(ctx context.Context, number string) (*model.CustodyTransfer, error) {
	var item model.CustodyTransfer
	err := r.db.WithContext(ctx).Where("transfer_no = ?", number).First(&item).Error
	return &item, err
}

func (r *transferRepository) CountPreparedForSpecimen(ctx context.Context, specimenID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.CustodyTransfer{}).
		Where("specimen_id = ? AND state = ?", specimenID, constants.TransferStatePrepared).Count(&count).Error
	return count, err
}

// expireStaleReservations 惰性释放已到期预约，保证同一格位只有一份有效预约。
func expireStaleReservations(tx *gorm.DB, now time.Time) error {
	return tx.Model(&model.SlotReservation{}).
		Where("state = ? AND expires_at <= ?", constants.ReservationActive, now).
		Updates(map[string]any{
			"state":          constants.ReservationExpired,
			"released_at":    now,
			"release_reason": "expired",
		}).Error
}

// CreateWithReservation 在单个事务内校验目标容器温区与容量、占用待交接格位并写入交接单和预约。
func (r *transferRepository) CreateWithReservation(ctx context.Context, item *model.CustodyTransfer, reservation *model.SlotReservation, specimen *model.Specimen) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := item.PreparedAt
		if err := expireStaleReservations(tx, now); err != nil {
			return err
		}
		var target model.StorageContainer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, *item.ToContainerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTargetContainerFull
			}
			return err
		}
		if !target.Active || target.Status != "available" {
			return ErrTargetContainerFull
		}
		if item.TemperatureC != nil && !target.AcceptsTemperature(*item.TemperatureC) {
			return ErrTemperatureExcursion
		}
		var activeReservations int64
		if err := tx.Model(&model.SlotReservation{}).
			Where("container_id = ? AND state = ? AND expires_at > ?", target.ID, constants.ReservationActive, now).
			Count(&activeReservations).Error; err != nil {
			return err
		}
		movingWithin := specimen.StorageContainerID != nil && *specimen.StorageContainerID == target.ID
		if !movingWithin && int64(target.Occupied)+activeReservations >= int64(target.Capacity) {
			return ErrTargetContainerFull
		}
		var reserved int64
		if err := tx.Model(&model.SlotReservation{}).
			Where("container_id = ? AND position = ? AND state = ? AND expires_at > ?", target.ID, item.ToPosition, constants.ReservationActive, now).
			Count(&reserved).Error; err != nil {
			return err
		}
		if reserved > 0 {
			return ErrSlotReservedByOther
		}
		var occupied int64
		if err := tx.Model(&model.Specimen{}).
			Where("storage_container_id = ? AND position = ? AND id <> ? AND state NOT IN ?", target.ID, item.ToPosition, specimen.ID, []constants.SpecimenState{constants.SpecimenStateReleased, constants.SpecimenStateDisposed}).
			Count(&occupied).Error; err != nil {
			return err
		}
		if occupied > 0 {
			return ErrPositionOccupied
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		reservation.TransferID = item.ID
		if err := tx.Create(reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) && strings.Contains(err.Error(), "slot_reservations") {
				return ErrSlotReservedByOther
			}
			return err
		}
		return nil
	})
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
		if err := expireStaleReservations(tx, now); err != nil {
			return err
		}
		var reservation model.SlotReservation
		hasReservation := true
		if err := tx.Where("transfer_id = ?", transfer.ID).First(&reservation).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			hasReservation = false
		}

		transfer.State = resolution.State
		if transfer.ToContainerID == nil || transfer.ToPosition == "" {
			transfer.ToContainerID = resolution.ToContainerID
			transfer.ToPosition = strings.TrimSpace(resolution.ToPosition)
		}
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
			var reserved int64
			if err := tx.Model(&model.SlotReservation{}).
				Where("container_id = ? AND position = ? AND state = ? AND expires_at > ? AND transfer_id <> ?", target.ID, transfer.ToPosition, constants.ReservationActive, now, transfer.ID).
				Count(&reserved).Error; err != nil {
				return err
			}
			if reserved > 0 {
				return ErrSlotReservedByOther
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
			if hasReservation && reservation.State == constants.ReservationActive {
				reservation.State = constants.ReservationConsumed
				reservation.ReleasedAt = &now
				reservation.ReleaseReason = "accepted"
				if err := tx.Save(&reservation).Error; err != nil {
					return err
				}
			}
		} else if hasReservation && reservation.State == constants.ReservationActive {
			reservation.State = constants.ReservationReleased
			reservation.ReleasedAt = &now
			reservation.ReleaseReason = string(resolution.State)
			if err := tx.Save(&reservation).Error; err != nil {
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

// SlotConflicts 计算待接收交接单目标格位的占用冲突，键为交接单 ID。
func (r *transferRepository) SlotConflicts(ctx context.Context, transfers []model.CustodyTransfer) (map[uint]string, error) {
	conflicts := make(map[uint]string)
	targets := make([]model.CustodyTransfer, 0, len(transfers))
	for _, item := range transfers {
		if item.State == constants.TransferStatePrepared && item.ToContainerID != nil && item.ToPosition != "" {
			targets = append(targets, item)
		}
	}
	if len(targets) == 0 {
		return conflicts, nil
	}
	pairCond := ""
	pairArgs := make([]any, 0, len(targets)*2)
	for i, item := range targets {
		if i > 0 {
			pairCond += " OR "
		}
		pairCond += "(container_id = ? AND position = ?)"
		pairArgs = append(pairArgs, *item.ToContainerID, item.ToPosition)
	}
	now := time.Now().UTC()
	reservations := make([]model.SlotReservation, 0)
	if err := r.db.WithContext(ctx).Preload("Transfer").Model(&model.SlotReservation{}).
		Where("state = ? AND expires_at > ?", constants.ReservationActive, now).
		Where(pairCond, pairArgs...).Find(&reservations).Error; err != nil {
		return nil, err
	}
	specimenCond := ""
	specimenArgs := make([]any, 0, len(targets)*2)
	for i, item := range targets {
		if i > 0 {
			specimenCond += " OR "
		}
		specimenCond += "(storage_container_id = ? AND position = ?)"
		specimenArgs = append(specimenArgs, *item.ToContainerID, item.ToPosition)
	}
	specimens := make([]model.Specimen, 0)
	if err := r.db.WithContext(ctx).Model(&model.Specimen{}).
		Select("id", "accession_no", "storage_container_id", "position").
		Where("state NOT IN ?", []constants.SpecimenState{constants.SpecimenStateReleased, constants.SpecimenStateDisposed}).
		Where(specimenCond, specimenArgs...).Find(&specimens).Error; err != nil {
		return nil, err
	}
	for _, item := range targets {
		for _, reservation := range reservations {
			if reservation.ContainerID == *item.ToContainerID && reservation.Position == item.ToPosition && reservation.TransferID != item.ID {
				number := "另一交接单"
				if reservation.Transfer != nil {
					number = reservation.Transfer.TransferNo
				}
				conflicts[item.ID] = fmt.Sprintf("目标格位已被交接单 %s 预约", number)
				break
			}
		}
		if conflicts[item.ID] != "" {
			continue
		}
		for _, occupant := range specimens {
			if occupant.StorageContainerID != nil && *occupant.StorageContainerID == *item.ToContainerID && occupant.Position == item.ToPosition && occupant.ID != item.SpecimenID {
				conflicts[item.ID] = fmt.Sprintf("目标格位已被样本 %s 占用", occupant.AccessionNo)
				break
			}
		}
	}
	return conflicts, nil
}
