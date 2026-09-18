package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type AuditHandler struct{ service service.AuditService }

func NewAuditHandler(auditService service.AuditService) *AuditHandler {
	return &AuditHandler{service: auditService}
}

func (h *AuditHandler) List(c *gin.Context) {
	var filter repository.AuditFilter
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
