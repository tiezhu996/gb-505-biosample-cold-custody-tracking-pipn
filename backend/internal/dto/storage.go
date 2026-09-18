package dto

type CreateStorageRequest struct {
	Code            string `json:"code" binding:"required,min=2,max=32"`
	Name            string `json:"name" binding:"required,min=2,max=100"`
	ContainerType   string `json:"containerType" binding:"required,min=2,max=80"`
	TemperatureZone string `json:"temperatureZone" binding:"required,oneof=minus20 minus80 liquid_nitrogen"`
	Location        string `json:"location" binding:"required,min=2,max=160"`
	Capacity        int    `json:"capacity" binding:"required,min=1,max=100000"`
	Status          string `json:"status" binding:"omitempty,oneof=available maintenance alarm"`
}

type UpdateStorageRequest struct {
	Name            *string `json:"name" binding:"omitempty,min=2,max=100"`
	ContainerType   *string `json:"containerType" binding:"omitempty,min=2,max=80"`
	TemperatureZone *string `json:"temperatureZone" binding:"omitempty,oneof=minus20 minus80 liquid_nitrogen"`
	Location        *string `json:"location" binding:"omitempty,min=2,max=160"`
	Capacity        *int    `json:"capacity" binding:"omitempty,min=1,max=100000"`
	Status          *string `json:"status" binding:"omitempty,oneof=available maintenance alarm"`
	Active          *bool   `json:"active"`
}
