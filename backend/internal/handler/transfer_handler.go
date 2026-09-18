package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type TransferHandler struct{ service service.TransferService }

func NewTransferHandler(transferService service.TransferService) *TransferHandler {
	return &TransferHandler{service: transferService}
}

func (h *TransferHandler) List(c *gin.Context) {
	var filter repository.TransferFilter
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

func (h *TransferHandler) Get(c *gin.Context) {
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

func (h *TransferHandler) Create(c *gin.Context) {
	var input dto.CreateTransferRequest
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

func (h *TransferHandler) Resolve(c *gin.Context) {
	id, ok := util.ParseID(c)
	if !ok {
		return
	}
	var input dto.ResolveTransferRequest
	if !util.BindJSON(c, &input) {
		return
	}
	item, err := h.service.Resolve(c.Request.Context(), ActorFromContext(c), id, input)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, item)
}
