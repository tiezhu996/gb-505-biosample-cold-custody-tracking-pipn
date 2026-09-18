package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/model"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type fakeTransferRepo struct {
	byNumberErr   error
	preparedCount int64
	createErr     error
	created       *model.CustodyTransfer
	stored        *model.CustodyTransfer
	resolveErr    error
	resolution    repository.TransferResolution
	resolveCalls  int
}

func (f *fakeTransferRepo) List(context.Context, repository.TransferFilter) ([]model.CustodyTransfer, int64, error) {
	return nil, 0, nil
}

func (f *fakeTransferRepo) Find(_ context.Context, id uint) (*model.CustodyTransfer, error) {
	if f.stored != nil && f.stored.ID == id {
		item := *f.stored
		return &item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeTransferRepo) FindByNumber(context.Context, string) (*model.CustodyTransfer, error) {
	return nil, f.byNumberErr
}

func (f *fakeTransferRepo) Create(_ context.Context, item *model.CustodyTransfer) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = item
	item.ID = 99
	f.stored = item
	return nil
}

func (f *fakeTransferRepo) CountPreparedForSpecimen(context.Context, uint) (int64, error) {
	return f.preparedCount, nil
}

func (f *fakeTransferRepo) Resolve(_ context.Context, id uint, resolution repository.TransferResolution) (*model.CustodyTransfer, *model.Specimen, model.Specimen, error) {
	f.resolveCalls++
	f.resolution = resolution
	if f.resolveErr != nil {
		return nil, nil, model.Specimen{}, f.resolveErr
	}
	resolved := *f.stored
	resolved.State = resolution.State
	return &resolved, &model.Specimen{}, model.Specimen{}, nil
}

type fakeSpecimenRepo struct{ specimen *model.Specimen }

func (f *fakeSpecimenRepo) List(context.Context, repository.SpecimenFilter) ([]model.Specimen, int64, error) {
	return nil, 0, nil
}

func (f *fakeSpecimenRepo) Find(_ context.Context, id uint) (*model.Specimen, error) {
	if f.specimen != nil && f.specimen.ID == id {
		item := *f.specimen
		return &item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeSpecimenRepo) FindForUpdate(context.Context, *gorm.DB, uint) (*model.Specimen, error) {
	return nil, nil
}

func (f *fakeSpecimenRepo) FindByAccession(context.Context, string) (*model.Specimen, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeSpecimenRepo) Create(context.Context, *model.Specimen) error { return nil }
func (f *fakeSpecimenRepo) Save(context.Context, *model.Specimen) error   { return nil }
func (f *fakeSpecimenRepo) Transaction(_ context.Context, fn func(*gorm.DB) error) error {
	return fn(nil)
}
func (f *fakeSpecimenRepo) Overview(context.Context) (*dto.CustodyOverview, error) { return nil, nil }

type fakeAudit struct{}

func (fakeAudit) Record(context.Context, Actor, string, string, uint, any, any) error { return nil }
func (fakeAudit) List(context.Context, repository.AuditFilter) (dto.PageResult[model.AuditLog], error) {
	return dto.PageResult[model.AuditLog]{}, nil
}
func (fakeAudit) Verify(context.Context) error { return nil }

func newTransferFixture() (*fakeTransferRepo, *transferService) {
	transferRepo := &fakeTransferRepo{byNumberErr: gorm.ErrRecordNotFound}
	specimenRepo := &fakeSpecimenRepo{specimen: &model.Specimen{
		AccessionNo: "BIO-20260822-001", SampleType: "血浆", SubjectCode: "SUBJ-T001",
		ProtocolCode: "PROTO-T-001", State: constants.SpecimenStateReceived,
		VolumeML: 4.5, CurrentCustodian: "接收员", ReceivedAt: time.Now().Add(-time.Hour),
	}}
	specimenRepo.specimen.ID = 11
	service := NewTransferService(transferRepo, specimenRepo, fakeAudit{})
	return transferRepo, service.(*transferService)
}

func createRequest(containerID *uint, position *string) dto.CreateTransferRequest {
	return dto.CreateTransferRequest{
		SpecimenID: 11, TransferNo: "TR-20260918-001", FromCustodian: "接收员", ToCustodian: "保管员",
		FromLocation: "intake", ToLocation: "样本库 B 区", ToContainerID: containerID, ToPosition: position,
	}
}

func TestCreateTransferRequiresContainerAndPositionTogether(t *testing.T) {
	_, service := newTransferFixture()
	actor := Actor{ID: 7, Name: "接收员", RequestID: "req-1"}
	containerID := uint(3)
	if _, err := service.Create(context.Background(), actor, createRequest(&containerID, nil)); err == nil {
		t.Fatal("reservation with container but no position must be rejected")
	}
	position := "R01-A01"
	if _, err := service.Create(context.Background(), actor, createRequest(nil, &position)); err == nil {
		t.Fatal("reservation with position but no container must be rejected")
	}
}

func TestCreateTransferBuildsThirtyMinuteReservation(t *testing.T) {
	repo, service := newTransferFixture()
	actor := Actor{ID: 7, Name: "接收员", RequestID: "req-1"}
	containerID := uint(3)
	position := "R01-A01"
	if _, err := service.Create(context.Background(), actor, createRequest(&containerID, &position)); err != nil {
		t.Fatalf("create with reservation failed: %v", err)
	}
	item := repo.created
	if item == nil || item.ReservationStatus != constants.ReservationActive {
		t.Fatalf("created transfer must hold an active reservation, got %+v", item)
	}
	if item.ReservedAt == nil || item.ReservationExpiresAt == nil {
		t.Fatal("reservation timestamps must be populated")
	}
	if delta := item.ReservationExpiresAt.Sub(*item.ReservedAt); delta != constants.ReservationTTL {
		t.Fatalf("reservation TTL = %v, want %v", delta, constants.ReservationTTL)
	}
	if item.ToContainerID == nil || *item.ToContainerID != containerID || item.ToPosition != position {
		t.Fatal("reservation must keep the requested container and position")
	}
}

func TestCreateTransferMapsReservationConflict(t *testing.T) {
	repo, service := newTransferFixture()
	repo.createErr = repository.ErrPositionReserved
	actor := Actor{ID: 7, Name: "接收员", RequestID: "req-1"}
	containerID := uint(3)
	position := "R01-A01"
	_, err := service.Create(context.Background(), actor, createRequest(&containerID, &position))
	var apiErr *util.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 409 {
		t.Fatalf("reservation conflict must map to 409, got %v", err)
	}
}

func reservedStoredTransfer(expiresAt time.Time) *model.CustodyTransfer {
	containerID := uint(3)
	reservedAt := expiresAt.Add(-constants.ReservationTTL)
	transfer := &model.CustodyTransfer{
		SpecimenID: 11, TransferNo: "TR-20260918-002", FromCustodian: "接收员", ToCustodian: "保管员",
		FromLocation: "intake", ToLocation: "样本库 B 区",
		ToContainerID: &containerID, ToPosition: "R01-A01",
		State: constants.TransferStatePrepared, PreparedByID: 1, PreparedByName: "接收员", PreparedAt: reservedAt,
		ReservationStatus: constants.ReservationActive, ReservedAt: &reservedAt, ReservationExpiresAt: &expiresAt,
	}
	transfer.ID = 21
	return transfer
}

func TestResolveAcceptUsesReservedPosition(t *testing.T) {
	repo, service := newTransferFixture()
	repo.stored = reservedStoredTransfer(time.Now().Add(10 * time.Minute))
	actor := Actor{ID: 2, Name: "保管员", RequestID: "req-2"}
	if _, err := service.Resolve(context.Background(), actor, 21, dto.ResolveTransferRequest{State: constants.TransferStateAccepted}); err != nil {
		t.Fatalf("accept with live reservation failed: %v", err)
	}
	if repo.resolveCalls != 1 {
		t.Fatal("repository resolve must be called exactly once")
	}
	if repo.resolution.ToContainerID == nil || *repo.resolution.ToContainerID != 3 || repo.resolution.ToPosition != "R01-A01" {
		t.Fatalf("resolution must reuse the reserved container and position, got %+v", repo.resolution)
	}
}

func TestResolveAcceptRejectsMismatchedTarget(t *testing.T) {
	repo, service := newTransferFixture()
	repo.stored = reservedStoredTransfer(time.Now().Add(10 * time.Minute))
	actor := Actor{ID: 2, Name: "保管员", RequestID: "req-2"}
	otherContainer := uint(9)
	otherPosition := "R09-Z99"
	for _, input := range []dto.ResolveTransferRequest{
		{State: constants.TransferStateAccepted, ToContainerID: &otherContainer},
		{State: constants.TransferStateAccepted, ToPosition: &otherPosition},
	} {
		if _, err := service.Resolve(context.Background(), actor, 21, input); err == nil {
			t.Fatal("accept with a target different from the reservation must fail")
		}
	}
	if repo.resolveCalls != 0 {
		t.Fatal("mismatched accept must not reach the repository")
	}
}

func TestResolveAcceptFailsAfterReservationExpiry(t *testing.T) {
	repo, service := newTransferFixture()
	repo.stored = reservedStoredTransfer(time.Now().Add(-time.Minute))
	actor := Actor{ID: 2, Name: "保管员", RequestID: "req-2"}
	_, err := service.Resolve(context.Background(), actor, 21, dto.ResolveTransferRequest{State: constants.TransferStateAccepted})
	var apiErr *util.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 409 {
		t.Fatalf("expired reservation accept must map to 409, got %v", err)
	}
	if repo.resolveCalls != 0 {
		t.Fatal("expired reservation accept must not reach the repository")
	}
}

func TestResolveMapsReservationErrors(t *testing.T) {
	for _, repoErr := range []error{repository.ErrPositionReserved, repository.ErrReservationExpired} {
		repo, service := newTransferFixture()
		repo.stored = reservedStoredTransfer(time.Now().Add(10 * time.Minute))
		repo.resolveErr = repoErr
		actor := Actor{ID: 2, Name: "保管员", RequestID: "req-2"}
		_, err := service.Resolve(context.Background(), actor, 21, dto.ResolveTransferRequest{State: constants.TransferStateAccepted})
		var apiErr *util.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != 409 {
			t.Fatalf("%v must map to 409, got %v", repoErr, err)
		}
	}
}
