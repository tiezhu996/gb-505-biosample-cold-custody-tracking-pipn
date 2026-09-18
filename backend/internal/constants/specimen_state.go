package constants

import "fmt"

type SpecimenState string

const (
	SpecimenStateReceived  SpecimenState = "received"
	SpecimenStateAliquoted SpecimenState = "aliquoted"
	SpecimenStateStored    SpecimenState = "stored"
	SpecimenStateReleased  SpecimenState = "released"
	SpecimenStateDisposed  SpecimenState = "disposed"
)

var specimenTransitions = map[SpecimenState]map[SpecimenState]struct{}{
	SpecimenStateReceived: {
		SpecimenStateAliquoted: {},
		SpecimenStateStored:    {},
		SpecimenStateDisposed:  {},
	},
	SpecimenStateAliquoted: {
		SpecimenStateStored:   {},
		SpecimenStateDisposed: {},
	},
	SpecimenStateStored: {
		SpecimenStateReleased: {},
		SpecimenStateDisposed: {},
	},
	SpecimenStateReleased: {},
	SpecimenStateDisposed: {},
}

func SpecimenStates() []SpecimenState {
	return []SpecimenState{
		SpecimenStateReceived,
		SpecimenStateAliquoted,
		SpecimenStateStored,
		SpecimenStateReleased,
		SpecimenStateDisposed,
	}
}

func (s SpecimenState) Valid() bool {
	_, ok := specimenTransitions[s]
	return ok
}

func (s SpecimenState) Terminal() bool {
	return s == SpecimenStateReleased || s == SpecimenStateDisposed
}

func (s SpecimenState) CanTransitionTo(next SpecimenState) bool {
	if s == next {
		return true
	}
	_, ok := specimenTransitions[s][next]
	return ok
}

func ValidateSpecimenTransition(current, next SpecimenState) error {
	if !current.Valid() {
		return fmt.Errorf("invalid current specimen state: %s", current)
	}
	if !next.Valid() {
		return fmt.Errorf("invalid target specimen state: %s", next)
	}
	if !current.CanTransitionTo(next) {
		return fmt.Errorf("specimen cannot transition from %s to %s", current, next)
	}
	return nil
}
