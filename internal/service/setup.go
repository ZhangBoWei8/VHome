package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"vhome/internal/auth"
	"vhome/internal/model"
	"vhome/internal/repository"
)

const (
	defaultHouseholdAvatar = "house"
	maxPasswordLength      = 128
)

type BootstrapState struct {
	Initialized bool
}

type SetupInput struct {
	HouseholdName     string
	HouseholdPassword string
	HouseholdAvatar   string

	OwnerName     string
	OwnerPassword string

	CreatedIP net.IP
	UserAgent string
}

type SetupResult struct {
	Household model.Household
	Owner     model.Member

	SessionToken     string
	CSRFToken        string
	SessionExpiresAt time.Time
}

func (s *IdentityService) GetBootstrapState(ctx context.Context) (BootstrapState, error) {
	exists, err := s.repository.HouseholdExists(ctx)
	if err != nil {
		return BootstrapState{}, fmt.Errorf(
			"get bootstrap state: %w",
			err,
		)
	}

	return BootstrapState{
		Initialized: exists,
	}, nil
}

func (s *IdentityService) Setup(ctx context.Context, input SetupInput) (SetupResult, error) {
	input = normalizeSetupInput(input)

	if err := validateSetupInput(input); err != nil {
		return SetupResult{}, err
	}

	exists, err := s.repository.HouseholdExists(ctx)
	if err != nil {
		return SetupResult{}, fmt.Errorf(
			"check initialization state: %w",
			err,
		)
	}

	if exists {
		return SetupResult{}, ErrAlreadyInitialized
	}

	householdPasswordHash, err := s.passwordHasher.Hash(input.HouseholdPassword)
	if err != nil {
		return SetupResult{}, fmt.Errorf(
			"hash household password: %w",
			err,
		)
	}

	ownerPasswordHash, err := s.passwordHasher.Hash(input.OwnerPassword)
	if err != nil {
		return SetupResult{}, fmt.Errorf(
			"hash owner password: %w",
			err,
		)
	}

	sessionToken, err := auth.NewToken()
	if err != nil {
		return SetupResult{}, fmt.Errorf(
			"generate session token: %w",
			err,
		)
	}

	csrfToken, err := auth.NewToken()
	if err != nil {
		return SetupResult{}, fmt.Errorf(
			"generate csrf token: %w",
			err,
		)
	}

	expiresAt := time.Now().UTC().Add(s.sessionTTL)

	var result SetupResult

	err = s.repository.WithinTransaction(
		ctx,
		func(txRepository *repository.Repository) error {
			exists, err := txRepository.HouseholdExists(ctx)
			if err != nil {
				return fmt.Errorf(
					"recheck initialization state: %w",
					err,
				)
			}

			if exists {
				return ErrAlreadyInitialized
			}

			household, err := txRepository.CreateHousehold(
				ctx,
				repository.CreateHouseholdParams{
					LoginName:   input.HouseholdName,
					DisplayName: input.HouseholdName,

					JoinSecretHash: householdPasswordHash,

					RegistrationEnabled: true,
					AvatarType:          "BUILTIN",
					AvatarValue:         input.HouseholdAvatar,
				},
			)
			if err != nil {
				if errors.Is(err, repository.ErrConflict) {
					return ErrAlreadyInitialized
				}

				return fmt.Errorf(
					"create household: %w",
					err,
				)
			}

			owner, err := txRepository.CreateMember(
				ctx,
				repository.CreateMemberParams{
					HouseholdID: household.ID,

					Username: input.OwnerName,

					UsernameNormalized: normalizeUsername(
						input.OwnerName,
					),

					DisplayName:  input.OwnerName,
					PasswordHash: ownerPasswordHash,

					Role:   model.MemberRoleOwner,
					Status: model.MemberStatusActive,
				},
			)
			if err != nil {
				return fmt.Errorf(
					"create household owner: %w",
					err,
				)
			}

			_, err = txRepository.CreateSession(
				ctx,
				repository.CreateSessionParams{
					HouseholdID: household.ID,
					MemberID:    owner.ID,

					TokenHash:     sessionToken.Hash,
					CSRFTokenHash: csrfToken.Hash,

					ExpiresAt: expiresAt,
					CreatedIP: input.CreatedIP,
					UserAgent: input.UserAgent,
				},
			)
			if err != nil {
				return fmt.Errorf(
					"create owner session: %w",
					err,
				)
			}

			result = SetupResult{
				Household: household,
				Owner:     owner,

				SessionToken:     sessionToken.Plaintext,
				CSRFToken:        csrfToken.Plaintext,
				SessionExpiresAt: expiresAt,
			}

			return nil
		},
	)
	if err != nil {
		if errors.Is(err, ErrAlreadyInitialized) {
			return SetupResult{}, ErrAlreadyInitialized
		}

		return SetupResult{}, fmt.Errorf(
			"setup vhome: %w",
			err,
		)
	}

	return result, nil
}

func normalizeSetupInput(input SetupInput) SetupInput {
	input.HouseholdName =
		strings.TrimSpace(input.HouseholdName)

	input.HouseholdAvatar =
		strings.TrimSpace(input.HouseholdAvatar)

	input.OwnerName =
		strings.TrimSpace(input.OwnerName)

	input.UserAgent =
		strings.TrimSpace(input.UserAgent)

	if input.HouseholdAvatar == "" {
		input.HouseholdAvatar = defaultHouseholdAvatar
	}

	return input
}

func normalizeUsername(username string) string {
	return strings.ToLower(
		strings.TrimSpace(username),
	)
}

func validateSetupInput(input SetupInput) error {
	if err := validateTextLength(
		"household_name",
		input.HouseholdName,
		1,
		64,
	); err != nil {
		return err
	}

	if err := validatePassword(
		"household_password",
		input.HouseholdPassword,
	); err != nil {
		return err
	}

	if err := validateTextLength(
		"household_avatar",
		input.HouseholdAvatar,
		1,
		64,
	); err != nil {
		return err
	}

	if err := validateTextLength(
		"owner_name",
		input.OwnerName,
		1,
		64,
	); err != nil {
		return err
	}

	if err := validatePassword(
		"owner_password",
		input.OwnerPassword,
	); err != nil {
		return err
	}

	return nil
}

func validateTextLength(field string, value string, minLength int, maxLength int) error {
	length := utf8.RuneCountInString(value)

	if length < minLength || length > maxLength {
		return fmt.Errorf(
			"%w: %s length must be between %d and %d",
			ErrInvalidInput,
			field,
			minLength,
			maxLength,
		)
	}

	return nil
}

func validatePassword(
	field string,
	password string,
) error {
	length := utf8.RuneCountInString(password)

	if length < 10 || length > maxPasswordLength {
		return fmt.Errorf(
			"%w: %s length must be between 10 and %d",
			ErrInvalidInput,
			field,
			maxPasswordLength,
		)
	}

	return nil
}
