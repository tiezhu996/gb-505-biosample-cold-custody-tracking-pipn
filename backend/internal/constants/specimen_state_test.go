package constants

import "testing"

func TestSpecimenTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    SpecimenState
		to      SpecimenState
		allowed bool
	}{
		{"received may be aliquoted", SpecimenStateReceived, SpecimenStateAliquoted, true},
		{"received may be stored", SpecimenStateReceived, SpecimenStateStored, true},
		{"aliquoted may be stored", SpecimenStateAliquoted, SpecimenStateStored, true},
		{"stored may be released", SpecimenStateStored, SpecimenStateReleased, true},
		{"received cannot be released", SpecimenStateReceived, SpecimenStateReleased, false},
		{"released is terminal", SpecimenStateReleased, SpecimenStateDisposed, false},
		{"disposed is terminal", SpecimenStateDisposed, SpecimenStateStored, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.from.CanTransitionTo(test.to); got != test.allowed {
				t.Fatalf("transition %s -> %s = %v, want %v", test.from, test.to, got, test.allowed)
			}
		})
	}
}

func TestTransferResolutionStates(t *testing.T) {
	for _, state := range []TransferState{TransferStateAccepted, TransferStateRejected, TransferStateCancelled} {
		if !TransferStatePrepared.CanResolveTo(state) {
			t.Fatalf("prepared transfer must resolve to %s", state)
		}
		if state.CanResolveTo(TransferStateAccepted) {
			t.Fatalf("resolved transfer %s must be terminal", state)
		}
	}
}

func TestRolePermissionsAreLeastPrivilege(t *testing.T) {
	if !RoleAdmin.Can("protocol:review") || !RoleAdmin.Can("storage:write") {
		t.Fatal("administrator must have every application permission")
	}
	if !RoleReceiver.Can("specimen:create") || RoleReceiver.Can("transfer:resolve") {
		t.Fatal("receiver grants are broader or narrower than expected")
	}
	if !RoleCustodian.Can("transfer:resolve") || RoleCustodian.Can("protocol:review") {
		t.Fatal("custodian must resolve transfers but not review protocols")
	}
	if !RoleReviewer.Can("protocol:review") || RoleReviewer.Can("specimen:create") {
		t.Fatal("reviewer must only receive review and audit grants")
	}
	if !RoleAuditor.Can("audit:read") || RoleAuditor.Can("specimen:update") {
		t.Fatal("auditor must be read-only")
	}
}
