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

type SpecimenService interface {
	List(context.Context, repository.SpecimenFilter) (dto.PageResult[model.Specimen], error)
	Get(context.Context, uint) (*model.Specimen, error)
	Create(context.Context, Actor, dto.CreateSpecimenRequest) (*model.Specimen, error)
	Update(context.Context, Actor, uint, dto.UpdateSpecimenRequest) (*model.Specimen, error)
	Transition(context.Context, Actor, uint, constants.SpecimenState, string) (*model.Specimen, error)
	Overview(context.Context) (*dto.CustodyOverview, error)
}

type specimenService struct {
	repo  repository.SpecimenRepository
	audit AuditService
}

func NewSpecimenService(repo repository.SpecimenRepository, audit AuditService) SpecimenService {
	return &specimenService{repo: repo, audit: audit}
}

func (s *specimenService) List(ctx context.Context, filter repository.SpecimenFilter) (dto.PageResult[model.Specimen], error) {
	query := filter.PageQuery.Normalize()
	filter.PageQuery = query
	items, total, err := s.repo.List(ctx, filter)
	return dto.PageResult[model.Specimen]{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

func (s *specimenService) Get(ctx context.Context, id uint) (*model.Specimen, error) {
	return s.repo.Find(ctx, id)
}

func (s *specimenService) Overview(ctx context.Context) (*dto.CustodyOverview, error) {
	return s.repo.Overview(ctx)
}

func (s *specimenService) Create(ctx context.Context, actor Actor, input dto.CreateSpecimenRequest) (*model.Specimen, error) {
	accession := strings.ToUpper(strings.TrimSpace(input.AccessionNo))
	if _, err := s.repo.FindByAccession(ctx, accession); err == nil {
		return nil, util.Conflict("样本接收号已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	receivedAt := time.Now().UTC()
	if input.ReceivedAt != nil {
		receivedAt = input.ReceivedAt.UTC()
	}
	if receivedAt.After(time.Now().Add(5 * time.Minute)) {
		return nil, util.BadRequest("接收时间不能晚于当前时间")
	}
	item := &model.Specimen{
		AccessionNo:      accession,
		SampleType:       input.SampleType,
		SubjectCode:      input.SubjectCode,
		ProtocolCode:     input.ProtocolCode,
		State:            constants.SpecimenStateReceived,
		VolumeML:         input.VolumeML,
		AliquotCount:     input.AliquotCount,
		CurrentCustodian: input.CurrentCustodian,
		ReceivedAt:       receivedAt,
		ExpiresAt:        input.ExpiresAt,
		Notes:            input.Notes,
	}
	item.Normalize()
	if err := item.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, actor, "specimen.received", "Specimen", item.ID, nil, item); err != nil {
		return nil, err
	}
	return s.repo.Find(ctx, item.ID)
}

func (s *specimenService) Update(ctx context.Context, actor Actor, id uint, input dto.UpdateSpecimenRequest) (*model.Specimen, error) {
	item, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if !item.Mutable() {
		return nil, util.Conflict("已销毁样本不可编辑")
	}
	before := *item
	if input.SampleType != nil {
		item.SampleType = *input.SampleType
	}
	if input.ProtocolCode != nil {
		if item.State != constants.SpecimenStateReceived {
			return nil, util.Conflict("样本进入分装或冻存流程后不能更换协议")
		}
		item.ProtocolCode = *input.ProtocolCode
	}
	if input.VolumeML != nil {
		item.VolumeML = *input.VolumeML
	}
	if input.AliquotCount != nil {
		item.AliquotCount = *input.AliquotCount
	}
	if input.CurrentCustodian != nil {
		if item.HasPreparedTransfer() {
			return nil, util.Conflict("样本存在待处理交接，不能直接修改保管人")
		}
		item.CurrentCustodian = *input.CurrentCustodian
	}
	if input.ExpiresAt != nil {
		item.ExpiresAt = input.ExpiresAt
	}
	if input.Notes != nil {
		item.Notes = *input.Notes
	}
	item.Normalize()
	if err := item.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, actor, "specimen.updated", "Specimen", item.ID, before, item); err != nil {
		return nil, err
	}
	return s.repo.Find(ctx, item.ID)
}

func (s *specimenService) Transition(ctx context.Context, actor Actor, id uint, next constants.SpecimenState, reason string) (*model.Specimen, error) {
	if next == constants.SpecimenStateReleased {
		return nil, util.Forbidden("样本放行只能由协议复核批准完成")
	}
	current, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *current
	if err := constants.ValidateSpecimenTransition(current.State, next); err != nil {
		return nil, util.Conflict(err.Error())
	}
	reason = strings.TrimSpace(reason)
	if next == constants.SpecimenStateDisposed && len([]rune(reason)) < 3 {
		return nil, util.BadRequest("销毁样本必须填写至少 3 个字符的原因")
	}
	if next == constants.SpecimenStateStored && current.StorageContainerID == nil {
		return nil, util.Conflict("样本必须通过交接受理进入冻存位置")
	}
	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		locked, lockErr := s.repo.FindForUpdate(ctx, tx, id)
		if lockErr != nil {
			return lockErr
		}
		if locked.State != current.State {
			return util.Conflict("样本状态已被其他请求更新")
		}
		locked.State = next
		if next == constants.SpecimenStateDisposed {
			if locked.StorageContainerID != nil {
				if updateErr := tx.Model(&model.StorageContainer{}).Where("id = ?", *locked.StorageContainerID).
					UpdateColumn("occupied", gorm.Expr("GREATEST(occupied - 1, 0)")).Error; updateErr != nil {
					return updateErr
				}
			}
			locked.StorageContainerID = nil
			locked.Position = ""
			locked.Notes = strings.TrimSpace(locked.Notes + "\n销毁原因: " + reason)
		}
		locked.Normalize()
		if validateErr := locked.Validate(); validateErr != nil {
			return util.BadRequest(validateErr.Error())
		}
		return tx.Save(locked).Error
	})
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, actor, "specimen.transitioned", "Specimen", id, before, updated); err != nil {
		return nil, err
	}
	return updated, nil
}
