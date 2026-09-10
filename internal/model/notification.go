package model

import "time"

type SMTPSecurity string

const (
	SMTPSecurityTLS      SMTPSecurity = "TLS"
	SMTPSecuritySTARTTLS SMTPSecurity = "STARTTLS"
)

func (s SMTPSecurity) Valid() bool {
	return s == SMTPSecurityTLS || s == SMTPSecuritySTARTTLS
}

const SMSProviderTencentCloud = "TENCENT_CLOUD"

// HouseholdNotificationSettings is internal domain data. Secret ciphertexts
// are deliberately excluded from JSON; HTTP responses use a separate view
// containing only configured/not-configured flags.
type HouseholdNotificationSettings struct {
	HouseholdID uint64

	EmailEnabled          bool
	SMTPHost              string
	SMTPPort              uint16
	SMTPSecurity          SMTPSecurity
	SMTPUsername          string
	SMTPPasswordEncrypted []byte
	SMTPFromEmail         string
	SMTPFromName          string

	SMSEnabled            bool
	SMSProvider           string
	SMSSecretID           string
	SMSSecretKeyEncrypted []byte
	SMSSDKAppID           string
	SMSSignName           string
	SMSTemplateID         string

	Version   uint64
	UpdatedBy *uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}
