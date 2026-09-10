package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vhome/internal/model"
)

const notificationSettingsColumns = `
	household_id,
	email_enabled,
	smtp_host,
	smtp_port,
	smtp_security,
	smtp_username,
	smtp_password_ciphertext,
	smtp_from_email,
	smtp_from_name,
	sms_enabled,
	sms_provider,
	sms_secret_id,
	sms_secret_key_ciphertext,
	sms_sdk_app_id,
	sms_sign_name,
	sms_template_id,
	version,
	updated_by,
	created_at,
	updated_at
`

type notificationSettingsScanner interface {
	Scan(dest ...any) error
}

func scanNotificationSettings(scanner notificationSettingsScanner) (model.HouseholdNotificationSettings, error) {
	var settings model.HouseholdNotificationSettings
	var smtpPassword, smsSecretKey []byte
	var updatedBy sql.NullInt64
	err := scanner.Scan(
		&settings.HouseholdID,
		&settings.EmailEnabled,
		&settings.SMTPHost,
		&settings.SMTPPort,
		&settings.SMTPSecurity,
		&settings.SMTPUsername,
		&smtpPassword,
		&settings.SMTPFromEmail,
		&settings.SMTPFromName,
		&settings.SMSEnabled,
		&settings.SMSProvider,
		&settings.SMSSecretID,
		&smsSecretKey,
		&settings.SMSSDKAppID,
		&settings.SMSSignName,
		&settings.SMSTemplateID,
		&settings.Version,
		&updatedBy,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return model.HouseholdNotificationSettings{}, err
	}
	settings.SMTPPasswordEncrypted = append([]byte(nil), smtpPassword...)
	settings.SMSSecretKeyEncrypted = append([]byte(nil), smsSecretKey...)
	if updatedBy.Valid {
		value := uint64(updatedBy.Int64)
		settings.UpdatedBy = &value
	}
	return settings, nil
}

func (r *Repository) GetHouseholdNotificationSettings(ctx context.Context, householdID uint64) (model.HouseholdNotificationSettings, error) {
	query := `SELECT ` + notificationSettingsColumns + `
		FROM household_notification_settings
		WHERE household_id = ?
		LIMIT 1`
	settings, err := scanNotificationSettings(r.q.QueryRowContext(ctx, query, householdID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HouseholdNotificationSettings{}, ErrNotFound
	}
	if err != nil {
		return model.HouseholdNotificationSettings{}, fmt.Errorf("get household notification settings: %w", err)
	}
	return settings, nil
}

func (r *Repository) CreateHouseholdNotificationSettings(ctx context.Context, settings model.HouseholdNotificationSettings) (model.HouseholdNotificationSettings, error) {
	const query = `
		INSERT INTO household_notification_settings (
			household_id, email_enabled, smtp_host, smtp_port, smtp_security,
			smtp_username, smtp_password_ciphertext, smtp_from_email, smtp_from_name,
			sms_enabled, sms_provider, sms_secret_id, sms_secret_key_ciphertext,
			sms_sdk_app_id, sms_sign_name, sms_template_id, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.q.ExecContext(ctx, query,
		settings.HouseholdID, settings.EmailEnabled, settings.SMTPHost, settings.SMTPPort,
		settings.SMTPSecurity, settings.SMTPUsername, nullableBytes(settings.SMTPPasswordEncrypted),
		settings.SMTPFromEmail, settings.SMTPFromName, settings.SMSEnabled,
		settings.SMSProvider, settings.SMSSecretID, nullableBytes(settings.SMSSecretKeyEncrypted),
		settings.SMSSDKAppID, settings.SMSSignName, settings.SMSTemplateID,
		nullableUint64(settings.UpdatedBy),
	)
	if isDuplicateKey(err) {
		return model.HouseholdNotificationSettings{}, ErrConflict
	}
	if err != nil {
		return model.HouseholdNotificationSettings{}, fmt.Errorf("create household notification settings: %w", err)
	}
	return r.GetHouseholdNotificationSettings(ctx, settings.HouseholdID)
}

func (r *Repository) UpdateHouseholdNotificationSettings(ctx context.Context, settings model.HouseholdNotificationSettings) (model.HouseholdNotificationSettings, error) {
	const query = `
		UPDATE household_notification_settings
		SET email_enabled = ?, smtp_host = ?, smtp_port = ?, smtp_security = ?,
			smtp_username = ?, smtp_password_ciphertext = ?, smtp_from_email = ?,
			smtp_from_name = ?, sms_enabled = ?, sms_provider = ?, sms_secret_id = ?,
			sms_secret_key_ciphertext = ?, sms_sdk_app_id = ?, sms_sign_name = ?,
			sms_template_id = ?, updated_by = ?, version = version + 1
		WHERE household_id = ? AND version = ?
	`
	result, err := r.q.ExecContext(ctx, query,
		settings.EmailEnabled, settings.SMTPHost, settings.SMTPPort, settings.SMTPSecurity,
		settings.SMTPUsername, nullableBytes(settings.SMTPPasswordEncrypted),
		settings.SMTPFromEmail, settings.SMTPFromName, settings.SMSEnabled,
		settings.SMSProvider, settings.SMSSecretID, nullableBytes(settings.SMSSecretKeyEncrypted),
		settings.SMSSDKAppID, settings.SMSSignName, settings.SMSTemplateID,
		nullableUint64(settings.UpdatedBy), settings.HouseholdID, settings.Version,
	)
	if err != nil {
		return model.HouseholdNotificationSettings{}, fmt.Errorf("update household notification settings: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return model.HouseholdNotificationSettings{}, fmt.Errorf("get notification settings affected rows: %w", err)
	}
	if affected != 1 {
		return model.HouseholdNotificationSettings{}, ErrConflict
	}
	return r.GetHouseholdNotificationSettings(ctx, settings.HouseholdID)
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
