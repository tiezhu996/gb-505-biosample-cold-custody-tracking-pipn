package constants

import (
	"testing"
	"time"
)

func TestReservationStatusValues(t *testing.T) {
	for _, status := range []ReservationStatus{ReservationNone, ReservationActive, ReservationConsumed, ReservationReleased, ReservationExpired} {
		if !status.Valid() {
			t.Fatalf("reservation status %q must be valid", status)
		}
	}
	if ReservationStatus("held").Valid() {
		t.Fatal("unknown reservation status must be invalid")
	}
}

func TestReservationStatusTerminal(t *testing.T) {
	if ReservationActive.Terminal() || ReservationNone.Terminal() {
		t.Fatal("active or empty reservation must not be terminal")
	}
	for _, status := range []ReservationStatus{ReservationConsumed, ReservationReleased, ReservationExpired} {
		if !status.Terminal() {
			t.Fatalf("reservation status %q must be terminal", status)
		}
	}
}

func TestReservationTTLIsThirtyMinutes(t *testing.T) {
	if ReservationTTL != 30*time.Minute {
		t.Fatalf("reservation TTL = %v, want 30m", ReservationTTL)
	}
}
