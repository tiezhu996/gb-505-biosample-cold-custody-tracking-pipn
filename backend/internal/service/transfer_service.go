package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type TransferService interface {
	List(context.Context, repository.TransferFilter) (dto.PageResult[model.CustodyTransfer], error)
	Get(context.Context, uint) (*model.CustodyTransfer, error)
	Create(context.Context, Actor, dto.CreateTransferRequest) (*model.CustodyTransfer, error)
	Resolve(context.Context, Actor, uint, dto.ResolveTransferRequest) (*model.CustodyTransfer, error)
}

type transferService struct {
	repo         repository.TransferRepository
	specimenRepo repository.SpecimenRepository
	audit        AuditService
}

func NewTransferService(repo repository.TransferRepository, specimenRepo repository.SpecimenRepository, audit AuditService) TransferService {
	return &transferService{repo: repo, specimenRepo: specimenRepo, audit: audit}
}

func (s *transferService) List(ctx context.Context, filter repository.TransferFilter) (dto.PageResult[model.CustodyTransfer], error) {
	query := filter.PageQuery.Normalize()
	filter.PageQuery = query
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return dto.PageResult[model.CustodyTransfer]{}, err
	}
	if err := s.decorate(ctx, items); err != nil {
		return dto.PageResult[model.CustodyTransfer]{}, err
	}
	return dto.PageResult[model.CustodyTransfer]{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *transferService) Get(ctx context.Context, id uint) (*model.CustodyTransfer, error) {
	item, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	items := []model.CustodyTransfer{*item}
	if err := s.decorate(ctx, items); err != nil {
		return nil, err
	}
	decorated := items[0]
	return &decorated, nil
}

// decorate 计算每份交接的预约有效状态和目标格位冲突原因，供交接页展示。
func (s *transferService) decorate(ctx context.Context, items []model.CustodyTransfer) error {
	now := time.Now().UTC()
	for i := range items {
		items[i].ReservationState = "none"
		if items[i].Reservation != nil {
			items[i].ReservationState = string(items[i].Reservation.EffectiveState(now))
		}
	}
	conflicts, err := s.repo.SlotConflicts(ctx, items)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].ConflictReason = conflictReason(&items[i], conflicts[items[i].ID])
	}
	return nil
}

func conflictReason(item *model.CustodyTransfer, slotConflict string) string {
	if item.State != constants.TransferStatePrepared || item.ToContainerID == nil || item.ToPosition == "" {
		return ""
	}
	if item.ToContainer == nil {
		return "目标容器不存在或已停用"
	}
	withinSameContainer := item.Specimen.StorageContainerID != nil && *item.Specimen.StorageContainerID == item.ToContainer.ID
	if !item.ToContainer.CanReceive() && !withinSameContainer {
		return "目标容器不可用或容量已满"
	}
	if item.TemperatureC != nil && !item.ToContainer.AcceptsTemperature(*item.TemperatureC) {
		return "交接温度超出目标容器温区"
	}
	return slotConflict
}

func (s *transferService) Create(ctx context.Context, actor Actor, input dto.CreateTransferRequest) (*model.CustodyTransfer, error) {
	number := strings.ToUpper(strings.TrimSpace(input.TransferNo))
	if _, err := s.repo.FindByNumber(ctx, number); err == nil {
		return nil, util.Conflict("交接单号已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	specimen, err := s.specimenRepo.Find(ctx, input.SpecimenID)
	if err != nil {
		return nil, err
	}
	if specimen.State.Terminal() {
		return nil, util.Conflict("已放行或已销毁样本不能发起交接")
	}
	prepared, err := s.repo.CountPreparedForSpecimen(ctx, specimen.ID)
	if err != nil {
		return nil, err
	}
	if prepared > 0 {
		return nil, util.Conflict("该样本已有待处理交接")
	}
	if strings.TrimSpace(input.FromCustodian) != specimen.CurrentCustodian {
		return nil, util.Conflict("交出人必须是样本当前保管人")
	}
	if strings.TrimSpace(actor.Name) != specimen.CurrentCustodian {
		return nil, util.Forbidden("只有样本当前保管人可以发起交接")
	}
	if strings.TrimSpace(input.FromLocation) != specimen.LocationLabel() {
		return nil, util.Conflict("交出位置与样本当前登记位置不一致")
	}
	if strings.TrimSpace(input.FromLocation) == strings.TrimSpace(input.ToLocation) && strings.TrimSpace(input.FromCustodian) == strings.TrimSpace(input.ToCustodian) {
		return nil, util.BadRequest("交接前后保管人或位置必须发生变化")
	}
	preparedAt := time.Now().UTC()
	item := &model.CustodyTransfer{
		SpecimenID:     specimen.ID,
		TransferNo:     number,
		FromCustodian:  input.FromCustodian,
		ToCustodian:    input.ToCustodian,
		FromLocation:   input.FromLocation,
		ToLocation:     input.ToLocation,
		ToContainerID:  input.ToContainerID,
		ToPosition:     strings.TrimSpace(input.ToPosition),
		State:          constants.TransferStatePrepared,
		PreparedByID:   actor.ID,
		PreparedByName: actor.Name,
		PreparedAt:     preparedAt,
		TemperatureC:   input.TemperatureC,
		Reason:         input.Reason,
	}
	item.Normalize()
	if err := item.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	reservation := &model.SlotReservation{
		ContainerID: *input.ToContainerID,
		Position:    item.ToPosition,
		State:       constants.ReservationActive,
		ExpiresAt:   preparedAt.Add(constants.SlotReservationTTL),
	}
	reservation.Normalize()
	if err := reservation.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	if err := s.repo.CreateWithReservation(ctx, item, reservation, specimen); err != nil {
		return nil, mapTransferError(err)
	}
	item.Reservation = reservation
	if err := s.audit.Record(ctx, actor, "custody_transfer.prepared", "CustodyTransfer", item.ID, nil, item); err != nil {
		return nil, err
	}
	return s.Get(ctx, item.ID)
}

func (s *transferService) Resolve(ctx context.Context, actor Actor, id uint, input dto.ResolveTransferRequest) (*model.CustodyTransfer, error) {
	beforeTransfer, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if !beforeTransfer.CanResolveTo(input.State) {
		return nil, util.Conflict("交接已经处理或目标状态无效")
	}
	if input.State == constants.TransferStateAccepted {
		if actor.ID == beforeTransfer.PreparedByID {
			return nil, util.Forbidden("交接发起人与接收人必须为不同人员")
		}
		if strings.TrimSpace(actor.Name) != beforeTransfer.ToCustodian {
			return nil, util.Forbidden("只有指定接收保管人可以受理交接")
		}
	}
	reason := strings.TrimSpace(input.Reason)
	if input.State != constants.TransferStateAccepted && len([]rune(reason)) < 3 {
		return nil, util.BadRequest("拒绝或取消交接必须填写至少 3 个字符的原因")
	}
	hasReservedTarget := beforeTransfer.ToContainerID != nil && *beforeTransfer.ToContainerID != 0 && strings.TrimSpace(beforeTransfer.ToPosition) != ""
	if input.State == constants.TransferStateAccepted && !hasReservedTarget {
		if input.ToContainerID == nil || *input.ToContainerID == 0 || input.ToPosition == nil || strings.TrimSpace(*input.ToPosition) == "" {
			return nil, util.BadRequest("受理交接必须选择目标容器和冻存位置")
		}
	}
	position := ""
	if input.ToPosition != nil {
		position = *input.ToPosition
	}
	targetContainerID := input.ToContainerID
	if input.State != constants.TransferStateAccepted {
		targetContainerID = nil
		position = ""
	}
	resolution := repository.TransferResolution{
		State:          input.State,
		ToContainerID:  targetContainerID,
		ToPosition:     position,
		TemperatureC:   input.TemperatureC,
		Reason:         reason,
		ResolvedByID:   actor.ID,
		ResolvedByName: actor.Name,
		ResolvedAt:     time.Now().UTC(),
	}
	resolved, specimen, specimenBefore, err := s.repo.Resolve(ctx, id, resolution)
	if err != nil {
		return nil, mapTransferError(err)
	}
	if err := s.audit.Record(ctx, actor, "custody_transfer."+string(input.State), "CustodyTransfer", id, beforeTransfer, resolved); err != nil {
		return nil, err
	}
	if input.State == constants.TransferStateAccepted {
		if err := s.audit.Record(ctx, actor, "specimen.relocated", "Specimen", specimen.ID, specimenBefore, specimen); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, resolved.ID)
}

func mapTransferError(err error) error {
	switch {
	case errors.Is(err, repository.ErrTransferAlreadyResolved):
		return util.Conflict("交接已被其他请求处理")
	case errors.Is(err, repository.ErrSpecimenCustodyChanged):
		return util.Conflict("样本位置或保管人已变化，请重新发起交接")
	case errors.Is(err, repository.ErrTargetContainerFull):
		return util.Conflict("目标容器不可用或容量已满")
	case errors.Is(err, repository.ErrPositionOccupied):
		return util.Conflict("目标冻存位置已被占用")
	case errors.Is(err, repository.ErrSlotReservedByOther):
		return util.Conflict("目标格位已被其他交接预约，本次操作未生效")
	case errors.Is(err, repository.ErrTemperatureExcursion):
		return util.Conflict("交接温度超出目标容器温区")
	default:
		return err
	}
}
