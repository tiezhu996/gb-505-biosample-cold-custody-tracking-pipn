package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type SpecimenHandler struct{ service service.SpecimenService }

func NewSpecimenHandler(specimenService service.SpecimenService) *SpecimenHandler {
	return &SpecimenHandler{service: specimenService}
}

func (h *SpecimenHandler) List(c *gin.Context) {
	var filter repository.SpecimenFilter
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

func (h *SpecimenHandler) Overview(c *gin.Context) {
	result, err := h.service.Overview(c.Request.Context())
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, result)
}

func (h *SpecimenHandler) Get(c *gin.Context) {
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

func (h *SpecimenHandler) Create(c *gin.Context) {
	var input dto.CreateSpecimenRequest
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

func (h *SpecimenHandler) Update(c *gin.Context) {
	id, ok := util.ParseID(c)
	if !ok {
		return
	}
	var input dto.UpdateSpecimenRequest
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

func (h *SpecimenHandler) Transition(c *gin.Context) {
	id, ok := util.ParseID(c)
	if !ok {
		return
	}
	var input dto.TransitionRequest
	if !util.BindJSON(c, &input) {
		return
	}
	item, err := h.service.Transition(c.Request.Context(), ActorFromContext(c), id, constants.SpecimenState(input.State), input.Reason)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, item)
}
