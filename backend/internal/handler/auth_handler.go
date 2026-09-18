package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"biosample-cold-custody-tracking/backend/internal/dto"
	"biosample-cold-custody-tracking/backend/internal/middleware"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

type AuthHandler struct{ service service.AuthService }

func NewAuthHandler(auth service.AuthService) *AuthHandler { return &AuthHandler{service: auth} }

func (h *AuthHandler) Login(c *gin.Context) {
	var input dto.LoginRequest
	if !util.BindJSON(c, &input) {
		return
	}
	response, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, response)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := c.Get(middleware.ContextUserID)
	if !ok {
		util.RespondError(c, util.Forbidden("无法识别当前用户"))
		return
	}
	user, err := h.service.CurrentUser(c.Request.Context(), userID.(uint))
	if err != nil {
		util.RespondError(c, err)
		return
	}
	util.Respond(c, http.StatusOK, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Access tokens are stateless and short-lived; the client discards the token.
	util.Respond(c, http.StatusOK, gin.H{"loggedOut": true})
}

func ActorFromContext(c *gin.Context) service.Actor {
	userID, _ := c.Get(middleware.ContextUserID)
	displayName, _ := c.Get(middleware.ContextDisplayName)
	actor := service.Actor{RequestID: c.GetString("requestId"), IP: c.ClientIP()}
	if id, ok := userID.(uint); ok {
		actor.ID = id
	}
	if name, ok := displayName.(string); ok {
		actor.Name = name
	}
	return actor
}
