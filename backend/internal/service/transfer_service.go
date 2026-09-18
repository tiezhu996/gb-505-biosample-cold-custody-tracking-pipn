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
	return dto.PageResult[model.CustodyTransfer]{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

func (s *transferService) Get(ctx context.Context, id uint) (*model.CustodyTransfer, error) {
	return s.repo.Find(ctx, id)
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
	item := &model.CustodyTransfer{
		SpecimenID:     specimen.ID,
		TransferNo:     number,
		FromCustodian:  input.FromCustodian,
		ToCustodian:    input.ToCustodian,
		FromLocation:   input.FromLocation,
		ToLocation:     input.ToLocation,
		State:          constants.TransferStatePrepared,
		PreparedByID:   actor.ID,
		PreparedByName: actor.Name,
		PreparedAt:     time.Now().UTC(),
		TemperatureC:   input.TemperatureC,
		Reason:         input.Reason,
	}
	item.Normalize()
	if err := item.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, actor, "custody_transfer.prepared", "CustodyTransfer", item.ID, nil, item); err != nil {
		return nil, err
	}
	return s.repo.Find(ctx, item.ID)
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
	if input.State == constants.TransferStateAccepted {
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
	return resolved, nil
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
	case errors.Is(err, repository.ErrTemperatureExcursion):
		return util.Conflict("交接温度超出目标容器温区")
	default:
		return err
	}
}
