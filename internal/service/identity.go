package service

import (
	"errors"
	"fmt"
	"time"

	"vhome/internal/auth"
	"vhome/internal/repository"
)

var (
	ErrInvalidInput             = errors.New("service: invalid input")
	ErrAlreadyInitialized       = errors.New("service: system already initialized")
	ErrNotInitialized           = errors.New("service: system not initialized")
	ErrRegistrationDisabled     = errors.New("service: registration disabled")
	ErrInvalidHouseholdPassword = errors.New("service: invalid household password")
	ErrUsernameAlreadyExists    = errors.New("service: username already exists")

	ErrInvalidCredentials    = errors.New("service: invalid credentials")
	ErrMemberPendingApproval = errors.New("service: member pending approval")
	ErrMemberRejected        = errors.New("service: member rejected")
	ErrMemberDisabled        = errors.New("service: member disabled")

	ErrUnauthenticated = errors.New("service: unauthenticated")

	ErrInvalidCSRFToken            = errors.New("service: invalid csrf token")
	ErrForbidden                   = errors.New("service: forbidden")
	ErrNotFound                    = errors.New("service: not found")
	ErrConflict                    = errors.New("service: conflict")
	ErrMaterialNameExists          = errors.New("service: material name already exists")
	ErrSecretEncryptionUnavailable = errors.New("service: secret encryption is not configured")
	ErrNotificationConfiguration   = errors.New("service: notification configuration is incomplete")
	ErrSMSProviderUnavailable      = errors.New("service: SMS provider is reserved but not enabled")
)

type IdentityService struct {
	repository     *repository.Repository
	passwordHasher *auth.PasswordHasher
	sessionTTL     time.Duration

	dummyPasswordHash string
}

func NewIdentityService(repo *repository.Repository, sessionTTL time.Duration) (*IdentityService, error) {
	if repo == nil {
		return nil, errors.New(
			"identity service repository is nil",
		)
	}

	if sessionTTL <= 0 {
		return nil, errors.New(
			"identity service session TTL must be positive",
		)
	}

	passwordHasher := auth.NewPasswordHasher()

	dummyPasswordHash, err := passwordHasher.Hash(
		"vhome-login-timing-padding-password",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create dummy password hash: %w",
			err,
		)
	}

	return &IdentityService{
		repository:        repo,
		passwordHasher:    passwordHasher,
		sessionTTL:        sessionTTL,
		dummyPasswordHash: dummyPasswordHash,
	}, nil
}
