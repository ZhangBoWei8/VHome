package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/middleware"
	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

type HomeHandler struct {
	identity  *service.IdentityService
	dashboard *service.DashboardService
}

func NewHomeHandler(identity *service.IdentityService, dashboard *service.DashboardService) *HomeHandler {
	return &HomeHandler{identity: identity, dashboard: dashboard}
}

func (h *HomeHandler) Dashboard(c *gin.Context) {
	actor, _ := middleware.CurrentIdentity(c)
	data, err := h.dashboard.Dashboard(c.Request.Context(), actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *HomeHandler) Notifications(c *gin.Context) {
	actor, _ := middleware.CurrentIdentity(c)
	data, err := h.dashboard.Notifications(c.Request.Context(), actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

type householdSettingsData struct {
	ID        uint64 `json:"id"`
	LoginName string `json:"login_name"`
	Name      string `json:"name"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Version   uint64 `json:"version"`
}

func newHouseholdSettingsData(household model.Household) householdSettingsData {
	return householdSettingsData{
		ID:        household.ID,
		LoginName: household.LoginName,
		Name:      household.DisplayName,
		Province:  household.Province,
		City:      household.City,
		Version:   household.Version,
	}
}

func (h *HomeHandler) Settings(c *gin.Context) {
	actor, _ := middleware.CurrentIdentity(c)
	household, err := h.identity.HouseholdSettings(c.Request.Context(), actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, newHouseholdSettingsData(household))
}

type updateHouseholdSettingsRequest struct {
	Name     string `json:"name" binding:"required"`
	Province string `json:"province"`
	City     string `json:"city"`
	Version  uint64 `json:"version" binding:"required"`
}

func (h *HomeHandler) UpdateSettings(c *gin.Context) {
	var request updateHouseholdSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	actor, _ := middleware.CurrentIdentity(c)
	household, err := h.identity.UpdateHouseholdSettings(
		c.Request.Context(),
		actor,
		service.UpdateHouseholdSettingsInput{
			Name: request.Name, Province: request.Province, City: request.City, Version: request.Version,
		},
	)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, newHouseholdSettingsData(household))
}

func (h *HomeHandler) Profile(c *gin.Context) {
	actor, _ := middleware.CurrentIdentity(c)
	member, err := h.identity.Profile(c.Request.Context(), actor)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, newMemberData(member))
}

type updateProfileRequest struct {
	DisplayName    string               `json:"display_name" binding:"required,max=64"`
	AvatarKey      model.MemberAvatar   `json:"avatar_key" binding:"required,max=32"`
	PresenceStatus model.PresenceStatus `json:"presence_status"`
	Email          string               `json:"email" binding:"omitempty,max=254"`
	Phone          string               `json:"phone" binding:"omitempty,max=32"`
	Version        uint64               `json:"version" binding:"required"`
}

func (h *HomeHandler) UpdateProfile(c *gin.Context) {
	var request updateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	actor, _ := middleware.CurrentIdentity(c)
	member, err := h.identity.UpdateProfile(c.Request.Context(), actor, service.UpdateProfileInput{
		DisplayName:    request.DisplayName,
		AvatarKey:      request.AvatarKey,
		PresenceStatus: request.PresenceStatus,
		Email:          request.Email,
		Phone:          request.Phone,
		Version:        request.Version,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, newMemberData(member))
}
