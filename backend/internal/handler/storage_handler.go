package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type StorageHandler struct{ service service.StorageService }

func NewStorageHandler(storageService service.StorageService) *StorageHandler {
	return &StorageHandler{service: storageService}
}

func (h *StorageHandler) List(c *gin.Context) {
	var filter repository.StorageFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		util.RespondError(c, util.BadRequest(err.Error()))
		return
	}
	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, result)
}

func (h *StorageHandler) Get(c *gin.Context) {
	id, ok := util.ParseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, item)
}

func (h *StorageHandler) Create(c *gin.Context) {
	var input dto.CreateStorageRequest
	if !util.BindJSON(c, &input) {
		return
	}
	item, err := h.service.Create(c.Request.Context(), ActorFromContext(c), input)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusCreated, item)
}

func (h *StorageHandler) Update(c *gin.Context) {
	id, ok := util.ParseID(c)
	if !ok {
		return
	}
	var input dto.UpdateStorageRequest
	if !util.BindJSON(c, &input) {
		return
	}
	item, err := h.service.Update(c.Request.Context(), ActorFromContext(c), id, input)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, item)
}
