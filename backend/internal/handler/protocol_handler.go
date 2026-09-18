package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type ProtocolHandler struct{ service service.ProtocolService }

func NewProtocolHandler(protocolService service.ProtocolService) *ProtocolHandler {
	return &ProtocolHandler{service: protocolService}
}

func (h *ProtocolHandler) List(c *gin.Context) {
	var filter repository.ProtocolFilter
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

func (h *ProtocolHandler) Get(c *gin.Context) {
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

func (h *ProtocolHandler) Review(c *gin.Context) {
	var input dto.CreateProtocolReviewRequest
	if !util.BindJSON(c, &input) {
		return
	}
	item, err := h.service.Review(c.Request.Context(), ActorFromContext(c), input)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusCreated, item)
}
