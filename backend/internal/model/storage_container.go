package model

import (
	"fmt"
	"regexp"
	"strings"
)

var containerCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,31}$`)

type StorageContainer struct {
	Base
	Code            string     `gorm:"size:32;uniqueIndex;not null" json:"code"`
	Name            string     `gorm:"size:100;not null" json:"name"`
	ContainerType   string     `gorm:"size:80;not null" json:"containerType"`
	TemperatureZone string     `gorm:"size:32;index;not null" json:"temperatureZone"`
	Location        string     `gorm:"size:160;not null" json:"location"`
	Capacity        int        `gorm:"not null" json:"capacity"`
	Occupied        int        `gorm:"not null;default:0" json:"occupied"`
	Status          string     `gorm:"size:24;index;not null;default:'available'" json:"status"`
	Active          bool       `gorm:"not null;default:true" json:"active"`
	Specimens       []Specimen `json:"specimens,omitempty"`
}

func (c *StorageContainer) Normalize() {
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	c.Name = strings.TrimSpace(c.Name)
	c.ContainerType = strings.TrimSpace(c.ContainerType)
	c.TemperatureZone = strings.ToLower(strings.TrimSpace(c.TemperatureZone))
	c.Location = strings.TrimSpace(c.Location)
	c.Status = strings.ToLower(strings.TrimSpace(c.Status))
	if c.Status == "" {
		c.Status = "available"
	}
}

func (c StorageContainer) Validate() error {
	if !containerCodePattern.MatchString(c.Code) {
		return fmt.Errorf("container code must contain 2-32 uppercase letters, numbers or hyphens")
	}
	if length := len([]rune(c.Name)); length < 2 || length > 100 {
		return fmt.Errorf("container name must contain 2-100 characters")
	}
	if length := len([]rune(c.ContainerType)); length < 2 || length > 80 {
		return fmt.Errorf("container type must contain 2-80 characters")
	}
	if !c.ValidTemperatureZone() {
		return fmt.Errorf("unsupported temperature zone: %s", c.TemperatureZone)
	}
	if length := len([]rune(c.Location)); length < 2 || length > 160 {
		return fmt.Errorf("location must contain 2-160 characters")
	}
	if c.Capacity < 1 || c.Capacity > 100000 {
		return fmt.Errorf("capacity must be between 1 and 100000")
	}
	if c.Occupied < 0 || c.Occupied > c.Capacity {
		return fmt.Errorf("occupied slots must be between zero and capacity")
	}
	if !c.ValidStatus() {
		return fmt.Errorf("unsupported container status: %s", c.Status)
	}
	return nil
}

func (c StorageContainer) ValidTemperatureZone() bool {
	switch c.TemperatureZone {
	case "minus20", "minus80", "liquid_nitrogen":
		return true
	default:
		return false
	}
}

func (c StorageContainer) ValidStatus() bool {
	switch c.Status {
	case "available", "maintenance", "alarm":
		return true
	default:
		return false
	}
}

func (c StorageContainer) AvailableSlots() int {
	remaining := c.Capacity - c.Occupied
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (c StorageContainer) CanReceive() bool {
	return c.Active && c.Status == "available" && c.AvailableSlots() > 0
}

func (c StorageContainer) AcceptsTemperature(value float64) bool {
	minimum, maximum := c.TemperatureRange()
	return value >= minimum && value <= maximum
}

func (c StorageContainer) TemperatureRange() (float64, float64) {
	switch c.TemperatureZone {
	case "minus20":
		return -30, -15
	case "minus80":
		return -90, -65
	case "liquid_nitrogen":
		return -196, -135
	default:
		return 1, 0
	}
}
