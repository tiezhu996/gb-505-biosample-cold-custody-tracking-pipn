package model

import "biosample-cold-custody-tracking/backend/internal/constants"

type User struct {
	Base
	Username     string         `gorm:"size:80;uniqueIndex;not null" json:"username"`
	DisplayName  string         `gorm:"size:100;not null" json:"displayName"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Role         constants.Role `gorm:"size:24;index;not null" json:"role"`
	Active       bool           `gorm:"not null;default:true" json:"active"`
}
