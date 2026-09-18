package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type ProtocolService interface {
	List(context.Context, repository.ProtocolFilter) (dto.PageResult[model.ProtocolReview], error)
	Get(context.Context, uint) (*model.ProtocolReview, error)
	Review(context.Context, Actor, dto.CreateProtocolReviewRequest) (*model.ProtocolReview, error)
}

type protocolService struct {
	repo         repository.ProtocolRepository
	specimenRepo repository.SpecimenRepository
	transferRepo repository.TransferRepository
	audit        AuditService
	objectStore  *minio.Client
	bucket       string
}

func NewProtocolService(repo repository.ProtocolRepository, specimenRepo repository.SpecimenRepository, transferRepo repository.TransferRepository, audit AuditService, objectStore *minio.Client, bucket string) ProtocolService {
	return &protocolService{repo: repo, specimenRepo: specimenRepo, transferRepo: transferRepo, audit: audit, objectStore: objectStore, bucket: bucket}
}

func (s *protocolService) List(ctx context.Context, filter repository.ProtocolFilter) (dto.PageResult[model.ProtocolReview], error) {
	query := filter.PageQuery.Normalize()
	filter.PageQuery = query
	items, total, err := s.repo.List(ctx, filter)
	return dto.PageResult[model.ProtocolReview]{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

func (s *protocolService) Get(ctx context.Context, id uint) (*model.ProtocolReview, error) {
	return s.repo.Find(ctx, id)
}

func (s *protocolService) Review(ctx context.Context, actor Actor, input dto.CreateProtocolReviewRequest) (*model.ProtocolReview, error) {
	if !input.Decision.Valid() {
		return nil, util.BadRequest("无效的协议复核决定")
	}
	specimen, err := s.specimenRepo.Find(ctx, input.SpecimenID)
	if err != nil {
		return nil, err
	}
	if strings.ToUpper(strings.TrimSpace(input.ProtocolCode)) != specimen.ProtocolCode {
		return nil, util.Conflict("复核协议与样本登记协议不一致")
	}
	prepared, err := s.transferRepo.CountPreparedForSpecimen(ctx, specimen.ID)
	if err != nil {
		return nil, err
	}
	if prepared > 0 {
		return nil, util.Conflict("样本仍有待处理交接，不能提交协议复核")
	}
	if input.Decision == constants.DecisionApproved && specimen.State != constants.SpecimenStateStored {
		return nil, util.Conflict("只有已冻存样本可以批准放行")
	}
	documentKey := ""
	if input.DocumentObjectKey != nil {
		documentKey = strings.TrimSpace(*input.DocumentObjectKey)
	}
	if documentKey != "" {
		if _, err := s.objectStore.StatObject(ctx, s.bucket, documentKey, minio.StatObjectOptions{}); err != nil {
			return nil, util.BadRequest("协议附件在对象存储中不存在")
		}
	}
	review := &model.ProtocolReview{
		SpecimenID:        specimen.ID,
		ProtocolCode:      input.ProtocolCode,
		Decision:          input.Decision,
		ReviewerID:        actor.ID,
		ReviewerName:      actor.Name,
		ConsentVerified:   input.ConsentVerified,
		ScopeVerified:     input.ScopeVerified,
		RetentionUntil:    input.RetentionUntil,
		DocumentObjectKey: documentKey,
		Notes:             input.Notes,
		ReviewedAt:        time.Now().UTC(),
	}
	review.Normalize()
	if err := review.Validate(); err != nil {
		return nil, util.BadRequest(err.Error())
	}
	created, afterSpecimen, beforeSpecimen, err := s.repo.Create(ctx, review)
	if errors.Is(err, repository.ErrSpecimenNotReviewable) {
		return nil, util.Conflict("样本状态或协议已变化，无法完成复核")
	}
	if err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, actor, "protocol_review.created", "ProtocolReview", created.ID, nil, created); err != nil {
		return nil, err
	}
	if input.Decision == constants.DecisionApproved {
		if err := s.audit.Record(ctx, actor, "specimen.released", "Specimen", specimen.ID, beforeSpecimen, afterSpecimen); err != nil {
			return nil, err
		}
	}
	return created, nil
}
