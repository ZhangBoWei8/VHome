package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"vhome/internal/auth"
	"vhome/internal/model"
	"vhome/internal/repository"
)

type NotificationService struct {
	repository *repository.Repository
	secretBox  *auth.SecretBox
}

func NewNotificationService(repo *repository.Repository, secretBox *auth.SecretBox) (*NotificationService, error) {
	if repo == nil {
		return nil, errors.New("notification service repository is nil")
	}
	if secretBox == nil {
		return nil, errors.New("notification service secret box is nil")
	}
	return &NotificationService{repository: repo, secretBox: secretBox}, nil
}

type NotificationSettingsInput struct {
	EmailEnabled  bool
	SMTPHost      string
	SMTPPort      uint16
	SMTPSecurity  model.SMTPSecurity
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string

	SMSEnabled    bool
	SMSSecretID   string
	SMSSecretKey  string
	SMSSDKAppID   string
	SMSSignName   string
	SMSTemplateID string
	Version       uint64
}

type NotificationSettingsView struct {
	EmailEnabled           bool               `json:"email_enabled"`
	SMTPHost               string             `json:"smtp_host"`
	SMTPPort               uint16             `json:"smtp_port"`
	SMTPSecurity           model.SMTPSecurity `json:"smtp_security"`
	SMTPUsername           string             `json:"smtp_username"`
	SMTPPasswordConfigured bool               `json:"smtp_password_configured"`
	SMTPFromEmail          string             `json:"smtp_from_email"`
	SMTPFromName           string             `json:"smtp_from_name"`

	SMSEnabled             bool   `json:"sms_enabled"`
	SMSProvider            string `json:"sms_provider"`
	SMSSecretID            string `json:"sms_secret_id"`
	SMSSecretKeyConfigured bool   `json:"sms_secret_key_configured"`
	SMSSDKAppID            string `json:"sms_sdk_app_id"`
	SMSSignName            string `json:"sms_sign_name"`
	SMSTemplateID          string `json:"sms_template_id"`
	SMSAvailable           bool   `json:"sms_available"`

	EncryptionAvailable bool   `json:"encryption_available"`
	Version             uint64 `json:"version"`
}

type MemoEmailDeliveryResult struct {
	Enabled  bool
	Eligible int
	Sent     int
	Failed   int
}

func defaultNotificationSettings(householdID uint64) model.HouseholdNotificationSettings {
	return model.HouseholdNotificationSettings{
		HouseholdID: householdID, SMTPPort: 465, SMTPSecurity: model.SMTPSecurityTLS,
		SMTPFromName: "VHome", SMSProvider: model.SMSProviderTencentCloud,
	}
}

func (s *NotificationService) Settings(ctx context.Context, actor AuthenticatedIdentity) (NotificationSettingsView, error) {
	if err := requireOwner(actor); err != nil {
		return NotificationSettingsView{}, err
	}
	settings, err := s.settings(ctx, actor.HouseholdID)
	if err != nil {
		return NotificationSettingsView{}, err
	}
	return s.settingsView(settings), nil
}

func (s *NotificationService) UpdateSettings(ctx context.Context, actor AuthenticatedIdentity, input NotificationSettingsInput) (NotificationSettingsView, error) {
	if err := requireOwner(actor); err != nil {
		return NotificationSettingsView{}, err
	}
	current, err := s.settings(ctx, actor.HouseholdID)
	if err != nil {
		return NotificationSettingsView{}, err
	}
	if input.Version != current.Version {
		return NotificationSettingsView{}, ErrConflict
	}

	settings := current
	settings.EmailEnabled = input.EmailEnabled
	settings.SMTPHost = strings.TrimSpace(input.SMTPHost)
	settings.SMTPPort = input.SMTPPort
	settings.SMTPSecurity = input.SMTPSecurity
	settings.SMTPUsername = strings.TrimSpace(input.SMTPUsername)
	settings.SMTPFromEmail = strings.TrimSpace(input.SMTPFromEmail)
	settings.SMTPFromName = strings.TrimSpace(input.SMTPFromName)
	settings.SMSEnabled = input.SMSEnabled
	settings.SMSProvider = model.SMSProviderTencentCloud
	settings.SMSSecretID = strings.TrimSpace(input.SMSSecretID)
	settings.SMSSDKAppID = strings.TrimSpace(input.SMSSDKAppID)
	settings.SMSSignName = strings.TrimSpace(input.SMSSignName)
	settings.SMSTemplateID = strings.TrimSpace(input.SMSTemplateID)
	settings.UpdatedBy = &actor.MemberID

	if strings.TrimSpace(input.SMTPPassword) != "" {
		settings.SMTPPasswordEncrypted, err = s.secretBox.Encrypt(input.SMTPPassword)
		if errors.Is(err, auth.ErrSecretEncryptionUnavailable) {
			return NotificationSettingsView{}, ErrSecretEncryptionUnavailable
		}
		if err != nil {
			return NotificationSettingsView{}, fmt.Errorf("encrypt SMTP password: %w", err)
		}
	}
	if strings.TrimSpace(input.SMSSecretKey) != "" {
		settings.SMSSecretKeyEncrypted, err = s.secretBox.Encrypt(input.SMSSecretKey)
		if errors.Is(err, auth.ErrSecretEncryptionUnavailable) {
			return NotificationSettingsView{}, ErrSecretEncryptionUnavailable
		}
		if err != nil {
			return NotificationSettingsView{}, fmt.Errorf("encrypt Tencent Cloud SMS secret: %w", err)
		}
	}
	if err := validateNotificationSettings(settings); err != nil {
		return NotificationSettingsView{}, err
	}
	if settings.SMSEnabled {
		return NotificationSettingsView{}, ErrSMSProviderUnavailable
	}

	var saved model.HouseholdNotificationSettings
	if current.Version == 0 {
		saved, err = s.repository.CreateHouseholdNotificationSettings(ctx, settings)
	} else {
		saved, err = s.repository.UpdateHouseholdNotificationSettings(ctx, settings)
	}
	if errors.Is(err, repository.ErrConflict) {
		return NotificationSettingsView{}, ErrConflict
	}
	if err != nil {
		return NotificationSettingsView{}, fmt.Errorf("save notification settings: %w", err)
	}
	return s.settingsView(saved), nil
}

func (s *NotificationService) TestEmail(ctx context.Context, actor AuthenticatedIdentity, recipient string) error {
	if err := requireOwner(actor); err != nil {
		return err
	}
	address, err := normalizeOptionalEmail(recipient)
	if err != nil || address == nil {
		return ErrInvalidInput
	}
	settings, err := s.settings(ctx, actor.HouseholdID)
	if err != nil {
		return err
	}
	config, err := s.smtpConfig(settings, false)
	if err != nil {
		return err
	}
	return sendSMTPMessage(ctx, config, *address, "VHome 邮件通知测试", "如果你收到这封邮件，说明家庭 SMTP 通知已经配置成功。", "<p>如果你收到这封邮件，说明家庭 SMTP 通知已经配置成功。 🌻</p>")
}

// DeliverMemoEmail sends one message per eligible recipient. It deliberately
// returns partial counts: the worker can avoid duplicating successful mail
// when only one family address is malformed or temporarily rejected.
func (s *NotificationService) DeliverMemoEmail(ctx context.Context, memo model.Memo) (MemoEmailDeliveryResult, error) {
	settings, err := s.settings(ctx, memo.HouseholdID)
	if err != nil {
		return MemoEmailDeliveryResult{}, err
	}
	if !settings.EmailEnabled {
		return MemoEmailDeliveryResult{}, nil
	}
	result := MemoEmailDeliveryResult{Enabled: true}
	config, err := s.smtpConfig(settings, true)
	if err != nil {
		return result, err
	}
	plainBody, htmlBody := memoEmailBodies(memo)
	var deliveryErrors []error
	for _, memberID := range memo.RecipientIDs {
		member, err := s.repository.GetMemberByID(ctx, memberID)
		if err != nil || member.HouseholdID != memo.HouseholdID || member.Status != model.MemberStatusActive || member.Email == nil {
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				deliveryErrors = append(deliveryErrors, fmt.Errorf("load memo recipient %d: %w", memberID, err))
			}
			continue
		}
		result.Eligible++
		var sendErr error
		for attempt := 1; attempt <= 3; attempt++ {
			attemptContext, cancel := context.WithTimeout(ctx, 20*time.Second)
			sendErr = sendSMTPMessage(attemptContext, config, *member.Email, "VHome 提醒 · "+memo.Title, plainBody, htmlBody)
			cancel()
			if sendErr == nil {
				break
			}
			if attempt < 3 {
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
				}
			}
		}
		if sendErr != nil {
			result.Failed++
			deliveryErrors = append(deliveryErrors, fmt.Errorf("send memo %d to member %d: %w", memo.ID, memberID, sendErr))
			continue
		}
		result.Sent++
	}
	return result, errors.Join(deliveryErrors...)
}

func (s *NotificationService) settings(ctx context.Context, householdID uint64) (model.HouseholdNotificationSettings, error) {
	settings, err := s.repository.GetHouseholdNotificationSettings(ctx, householdID)
	if errors.Is(err, repository.ErrNotFound) {
		return defaultNotificationSettings(householdID), nil
	}
	if err != nil {
		return model.HouseholdNotificationSettings{}, fmt.Errorf("get notification settings: %w", err)
	}
	return settings, nil
}

func (s *NotificationService) settingsView(settings model.HouseholdNotificationSettings) NotificationSettingsView {
	return NotificationSettingsView{
		EmailEnabled: settings.EmailEnabled, SMTPHost: settings.SMTPHost,
		SMTPPort: settings.SMTPPort, SMTPSecurity: settings.SMTPSecurity,
		SMTPUsername:           settings.SMTPUsername,
		SMTPPasswordConfigured: len(settings.SMTPPasswordEncrypted) > 0,
		SMTPFromEmail:          settings.SMTPFromEmail, SMTPFromName: settings.SMTPFromName,
		SMSEnabled: settings.SMSEnabled, SMSProvider: settings.SMSProvider,
		SMSSecretID:            settings.SMSSecretID,
		SMSSecretKeyConfigured: len(settings.SMSSecretKeyEncrypted) > 0,
		SMSSDKAppID:            settings.SMSSDKAppID, SMSSignName: settings.SMSSignName,
		SMSTemplateID: settings.SMSTemplateID, SMSAvailable: false,
		EncryptionAvailable: s.secretBox.Available(), Version: settings.Version,
	}
}

func validateNotificationSettings(settings model.HouseholdNotificationSettings) error {
	if settings.SMTPPort == 0 || !settings.SMTPSecurity.Valid() || len([]rune(settings.SMTPHost)) > 255 ||
		len([]rune(settings.SMTPUsername)) > 254 || len([]rune(settings.SMTPFromName)) > 128 {
		return ErrInvalidInput
	}
	if settings.SMTPFromEmail != "" {
		if _, err := normalizeOptionalEmail(settings.SMTPFromEmail); err != nil {
			return err
		}
	}
	if settings.EmailEnabled && (settings.SMTPHost == "" || settings.SMTPUsername == "" ||
		settings.SMTPFromEmail == "" || len(settings.SMTPPasswordEncrypted) == 0) {
		return ErrNotificationConfiguration
	}
	return nil
}

func (s *NotificationService) smtpConfig(settings model.HouseholdNotificationSettings, requireEnabled bool) (smtpConfig, error) {
	if requireEnabled && !settings.EmailEnabled {
		return smtpConfig{}, ErrNotificationConfiguration
	}
	if err := validateNotificationSettings(settings); err != nil {
		return smtpConfig{}, err
	}
	if len(settings.SMTPPasswordEncrypted) == 0 {
		return smtpConfig{}, ErrNotificationConfiguration
	}
	password, err := s.secretBox.Decrypt(settings.SMTPPasswordEncrypted)
	if errors.Is(err, auth.ErrSecretEncryptionUnavailable) {
		return smtpConfig{}, ErrSecretEncryptionUnavailable
	}
	if err != nil {
		return smtpConfig{}, fmt.Errorf("decrypt SMTP password: %w", err)
	}
	return smtpConfig{
		Host: settings.SMTPHost, Port: settings.SMTPPort, Security: settings.SMTPSecurity,
		Username: settings.SMTPUsername, Password: password,
		FromEmail: settings.SMTPFromEmail, FromName: settings.SMTPFromName,
	}, nil
}
