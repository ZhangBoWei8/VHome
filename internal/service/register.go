package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vhome/internal/model"
	"vhome/internal/repository"
)

type RegisterMemberInput struct {
	Name              string
	Password          string
	HouseholdPassword string
}

type RegisterMemberResult struct {
	Member model.Member
}

func (s *IdentityService) RegisterMember(ctx context.Context, input RegisterMemberInput) (RegisterMemberResult, error) {
	input = normalizeRegisterMemberInput(input)

	if err := validateRegisterMemberInput(input); err != nil {
		return RegisterMemberResult{}, err
	}

	household, err := s.repository.GetHousehold(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return RegisterMemberResult{}, ErrNotInitialized
		}

		return RegisterMemberResult{}, fmt.Errorf(
			"get household for registration: %w",
			err,
		)
	}

	if !household.RegistrationEnabled {
		return RegisterMemberResult{}, ErrRegistrationDisabled
	}

	validHouseholdPassword, err := s.passwordHasher.Verify(
		input.HouseholdPassword,
		household.JoinSecretHash,
	)
	if err != nil {
		return RegisterMemberResult{}, fmt.Errorf(
			"verify household password: %w",
			err,
		)
	}

	if !validHouseholdPassword {
		return RegisterMemberResult{},
			ErrInvalidHouseholdPassword
	}

	passwordHash, err := s.passwordHasher.Hash(
		input.Password,
	)
	if err != nil {
		return RegisterMemberResult{}, fmt.Errorf(
			"hash member password: %w",
			err,
		)
	}

	member, err := s.repository.CreateMember(
		ctx,
		repository.CreateMemberParams{
			HouseholdID: household.ID,

			Username: input.Name,

			UsernameNormalized: normalizeUsername(
				input.Name,
			),

			DisplayName:  input.Name,
			PasswordHash: passwordHash,

			Role:   model.MemberRoleMember,
			Status: model.MemberStatusPending,
		},
	)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return RegisterMemberResult{},
				ErrUsernameAlreadyExists
		}

		return RegisterMemberResult{}, fmt.Errorf(
			"create pending member: %w",
			err,
		)
	}

	return RegisterMemberResult{
		Member: member,
	}, nil
}

func normalizeRegisterMemberInput(input RegisterMemberInput) RegisterMemberInput {
	input.Name = strings.TrimSpace(input.Name)

	return input
}

func validateRegisterMemberInput(input RegisterMemberInput) error {
	if err := validateTextLength(
		"name",
		input.Name,
		1,
		64,
	); err != nil {
		return err
	}

	if err := validatePassword(
		"password",
		input.Password,
	); err != nil {
		return err
	}

	if err := validatePassword(
		"household_password",
		input.HouseholdPassword,
	); err != nil {
		return err
	}

	return nil
}
