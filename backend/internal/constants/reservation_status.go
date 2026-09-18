package constants

import "time"

// ReservationTTL 是目标格位预约的有效时长，超时未接收的预约自动释放。
const ReservationTTL = 30 * time.Minute

type ReservationStatus string

const (
	ReservationNone     ReservationStatus = ""
	ReservationActive   ReservationStatus = "active"
	ReservationConsumed ReservationStatus = "consumed"
	ReservationReleased ReservationStatus = "released"
	ReservationExpired  ReservationStatus = "expired"
)

func (s ReservationStatus) Valid() bool {
	switch s {
	case ReservationNone, ReservationActive, ReservationConsumed, ReservationReleased, ReservationExpired:
		return true
	default:
		return false
	}
}

func (s ReservationStatus) Terminal() bool {
	return s == ReservationConsumed || s == ReservationReleased || s == ReservationExpired
}
