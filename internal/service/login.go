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

type LoginInput struct {
	Name     string
	Password string

	CreatedIP net.IP
	UserAgent string
}

type LoginResult struct {
	Household model.Household
	Member    model.Member

	SessionToken     string
	CSRFToken        string
	SessionExpiresAt time.Time
}

func (s *IdentityService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	input = normalizeLoginInput(input)

	if err := validateLoginInput(input); err != nil {
		return LoginResult{}, err
	}

	household, err := s.repository.GetHousehold(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return LoginResult{}, ErrNotInitialized
		}

		return LoginResult{}, fmt.Errorf(
			"get household for login: %w",
			err,
		)
	}

	member, err := s.repository.GetMemberByUsername(
		ctx,
		household.ID,
		normalizeUsername(input.Name),
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.consumeDummyPasswordCheck(input.Password)

			return LoginResult{}, ErrInvalidCredentials
		}

		return LoginResult{}, fmt.Errorf(
			"get login member: %w",
			err,
		)
	}

	passwordValid, err := s.passwordHasher.Verify(input.Password, member.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"verify member password: %w",
			err,
		)
	}

	if !passwordValid {
		return LoginResult{}, ErrInvalidCredentials
	}

	if err := validateMemberLoginStatus(member.Status); err != nil {
		return LoginResult{}, err
	}

	sessionToken, err := auth.NewToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"generate login session token: %w",
			err,
		)
	}

	csrfToken, err := auth.NewToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"generate login csrf token: %w",
			err,
		)
	}

	expiresAt := time.Now().UTC().Add(s.sessionTTL)

	var authenticatedMember model.Member

	err = s.repository.WithinTransaction(
		ctx,
		func(txRepository *repository.Repository) error {
			currentMember, err :=
				txRepository.GetMemberByID(ctx, member.ID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return ErrInvalidCredentials
				}

				return fmt.Errorf(
					"reload login member: %w",
					err,
				)
			}

			if currentMember.HouseholdID != household.ID {
				return ErrInvalidCredentials
			}

			if currentMember.PasswordHash != member.PasswordHash {
				return ErrInvalidCredentials
			}

			if err := validateMemberLoginStatus(
				currentMember.Status,
			); err != nil {
				return err
			}

			if !currentMember.Role.Valid() {
				return fmt.Errorf(
					"member has invalid role: %q",
					currentMember.Role,
				)
			}

			_, err = txRepository.CreateSession(
				ctx,
				repository.CreateSessionParams{
					HouseholdID: household.ID,
					MemberID:    currentMember.ID,

					TokenHash:     sessionToken.Hash,
					CSRFTokenHash: csrfToken.Hash,

					ExpiresAt: expiresAt,
					CreatedIP: input.CreatedIP,
					UserAgent: input.UserAgent,
				},
			)
			if err != nil {
				return fmt.Errorf(
					"create login session: %w",
					err,
				)
			}

			if err := txRepository.UpdateMemberLastLogin(
				ctx,
				currentMember.ID,
			); err != nil {
				return fmt.Errorf(
					"update member last login: %w",
					err,
				)
			}

			authenticatedMember = currentMember

			return nil
		},
	)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Household: household,
		Member:    authenticatedMember,

		SessionToken:     sessionToken.Plaintext,
		CSRFToken:        csrfToken.Plaintext,
		SessionExpiresAt: expiresAt,
	}, nil
}

func (s *IdentityService) consumeDummyPasswordCheck(password string) {
	_, _ = s.passwordHasher.Verify(
		password,
		s.dummyPasswordHash,
	)
}

// 规范化登录输入
func normalizeLoginInput(input LoginInput) LoginInput {
	input.Name = strings.TrimSpace(input.Name)
	input.UserAgent = strings.TrimSpace(input.UserAgent)

	return input
}

// 验证登录输入
func validateLoginInput(input LoginInput) error {
	if err := validateTextLength(
		"name",
		input.Name,
		1,
		64,
	); err != nil {
		return err
	}

	passwordLength := utf8.RuneCountInString(
		input.Password,
	)

	if passwordLength < 1 ||
		passwordLength > maxPasswordLength {
		return fmt.Errorf(
			"%w: password length must be between 1 and %d",
			ErrInvalidInput,
			maxPasswordLength,
		)
	}

	if utf8.RuneCountInString(input.UserAgent) > 512 {
		return fmt.Errorf(
			"%w: user_agent length must not exceed 512",
			ErrInvalidInput,
		)
	}

	return nil
}

func validateMemberLoginStatus(status model.MemberStatus) error {
	switch status {
	case model.MemberStatusActive:
		return nil

	case model.MemberStatusPending:
		return ErrMemberPendingApproval

	case model.MemberStatusRejected:
		return ErrMemberRejected

	case model.MemberStatusDisabled:
		return ErrMemberDisabled

	default:
		return fmt.Errorf(
			"member has invalid status: %q",
			status,
		)
	}
}
