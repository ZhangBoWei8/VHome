package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

type NotificationHandler struct {
	service *service.NotificationService
}

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: notificationService}
}

type notificationSettingsRequest struct {
	EmailEnabled  bool               `json:"email_enabled"`
	SMTPHost      string             `json:"smtp_host"`
	SMTPPort      uint16             `json:"smtp_port"`
	SMTPSecurity  model.SMTPSecurity `json:"smtp_security"`
	SMTPUsername  string             `json:"smtp_username"`
	SMTPPassword  string             `json:"smtp_password"`
	SMTPFromEmail string             `json:"smtp_from_email"`
	SMTPFromName  string             `json:"smtp_from_name"`

	SMSEnabled    bool   `json:"sms_enabled"`
	SMSSecretID   string `json:"sms_secret_id"`
	SMSSecretKey  string `json:"sms_secret_key"`
	SMSSDKAppID   string `json:"sms_sdk_app_id"`
	SMSSignName   string `json:"sms_sign_name"`
	SMSTemplateID string `json:"sms_template_id"`
	Version       uint64 `json:"version"`
}

func (h *NotificationHandler) Settings(c *gin.Context) {
	data, err := h.service.Settings(c.Request.Context(), currentActor(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

func (h *NotificationHandler) UpdateSettings(c *gin.Context) {
	var request notificationSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	data, err := h.service.UpdateSettings(c.Request.Context(), currentActor(c), service.NotificationSettingsInput{
		EmailEnabled: request.EmailEnabled, SMTPHost: request.SMTPHost,
		SMTPPort: request.SMTPPort, SMTPSecurity: request.SMTPSecurity,
		SMTPUsername: request.SMTPUsername, SMTPPassword: request.SMTPPassword,
		SMTPFromEmail: request.SMTPFromEmail, SMTPFromName: request.SMTPFromName,
		SMSEnabled: request.SMSEnabled, SMSSecretID: request.SMSSecretID,
		SMSSecretKey: request.SMSSecretKey, SMSSDKAppID: request.SMSSDKAppID,
		SMSSignName: request.SMSSignName, SMSTemplateID: request.SMSTemplateID,
		Version: request.Version,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.WriteData(c, http.StatusOK, data)
}

type testEmailRequest struct {
	Recipient string `json:"recipient" binding:"required,max=254"`
}

func (h *NotificationHandler) TestEmail(c *gin.Context) {
	var request testEmailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}
	if err := h.service.TestEmail(c.Request.Context(), currentActor(c), request.Recipient); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
