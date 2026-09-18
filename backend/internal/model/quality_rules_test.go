package model

import (
	"testing"
	"time"

	"biosample-cold-custody-tracking/backend/internal/constants"
)

func validSpecimen() Specimen {
	received := time.Now().Add(-time.Hour)
	expires := received.AddDate(1, 0, 0)
	return Specimen{
		AccessionNo:      "BIO-20260822-TEST",
		SampleType:       "血浆",
		SubjectCode:      "SUBJ-T001",
		ProtocolCode:     "PROTO-TEST-001",
		State:            constants.SpecimenStateReceived,
		VolumeML:         4.5,
		AliquotCount:     0,
		CurrentCustodian: "样本接收员",
		ReceivedAt:       received,
		ExpiresAt:        &expires,
	}
}

func TestSpecimenValidationTracksStorageInvariant(t *testing.T) {
	specimen := validSpecimen()
	if err := specimen.Validate(); err != nil {
		t.Fatalf("valid received specimen rejected: %v", err)
	}
	specimen.State = constants.SpecimenStateStored
	if err := specimen.Validate(); err == nil {
		t.Fatal("stored specimen without location must be rejected")
	}
	containerID := uint(7)
	specimen.StorageContainerID = &containerID
	specimen.Position = "R01-BX02-A03"
	if err := specimen.Validate(); err != nil {
		t.Fatalf("located stored specimen rejected: %v", err)
	}
}

func TestStorageContainerTemperatureAndCapacity(t *testing.T) {
	container := StorageContainer{
		Code: "FZ-80-01", Name: "负八十度一号柜", ContainerType: "ultra_low_freezer",
		TemperatureZone: "minus80", Location: "B 区", Capacity: 20, Occupied: 19,
		Status: "available", Active: true,
	}
	if err := container.Validate(); err != nil {
		t.Fatalf("valid storage container rejected: %v", err)
	}
	if !container.CanReceive() || !container.AcceptsTemperature(-78.5) {
		t.Fatal("available minus80 container should accept an in-range transfer")
	}
	if container.AcceptsTemperature(-20) {
		t.Fatal("minus80 container must reject a warm transfer")
	}
	container.Occupied = container.Capacity
	if container.CanReceive() {
		t.Fatal("full container must not accept another specimen")
	}
}

func TestCustodyTransferResolutionValidation(t *testing.T) {
	prepared := time.Now().Add(-time.Minute)
	transfer := CustodyTransfer{
		SpecimenID: 1, TransferNo: "CT-20260822-001", FromCustodian: "样本接收员",
		ToCustodian: "冻存保管员", FromLocation: "intake", ToLocation: "样本库 B 区",
		State: constants.TransferStatePrepared, PreparedByID: 2, PreparedByName: "接收员", PreparedAt: prepared,
	}
	if err := transfer.Validate(); err != nil {
		t.Fatalf("valid prepared transfer rejected: %v", err)
	}
	now := time.Now()
	resolverID := uint(3)
	containerID := uint(9)
	temperature := -79.2
	transfer.State = constants.TransferStateAccepted
	transfer.AcceptedByID = &resolverID
	transfer.AcceptedByName = "保管员"
	transfer.ResolvedAt = &now
	transfer.ToContainerID = &containerID
	transfer.ToPosition = "R02-BX03-D04"
	transfer.TemperatureC = &temperature
	if err := transfer.Validate(); err != nil {
		t.Fatalf("valid accepted transfer rejected: %v", err)
	}
}

func TestProtocolApprovalRequiresConsentAndScope(t *testing.T) {
	review := ProtocolReview{
		SpecimenID: 1, ProtocolCode: "PROTO-TEST-001", Decision: constants.DecisionApproved,
		ReviewerID: 4, ReviewerName: "复核员", ReviewedAt: time.Now(),
	}
	if err := review.Validate(); err == nil {
		t.Fatal("approval without consent and scope verification must fail")
	}
	review.ConsentVerified = true
	review.ScopeVerified = true
	if err := review.Validate(); err != nil {
		t.Fatalf("verified approval rejected: %v", err)
	}
}

func TestAuditEntryHashDetectsMutation(t *testing.T) {
	entry := AuditLog{
		CreatedAt: time.Now().UTC(), RequestID: "req-001", ActorID: 3, ActorName: "保管员",
		Action: "specimen.relocated", EntityType: "Specimen", EntityID: 10,
		BeforeState: "{}", AfterState: "{}", BeforeLocation: "intake", AfterLocation: "B 区",
		BeforeCustodian: "接收员", AfterCustodian: "保管员", IPAddress: "127.0.0.1",
	}
	entry.Seal("previous-hash")
	if !entry.IntegrityValid() {
		t.Fatal("sealed audit entry must pass integrity check")
	}
	entry.AfterLocation = "tampered"
	if entry.IntegrityValid() {
		t.Fatal("mutated audit entry must fail integrity check")
	}
}
