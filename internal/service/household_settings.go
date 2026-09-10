package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"vhome/internal/model"
	"vhome/internal/repository"
)

type UpdateHouseholdSettingsInput struct {
	Name     string
	Province string
	City     string
	Version  uint64
}

type UpdateProfileInput struct {
	DisplayName    string
	AvatarKey      model.MemberAvatar
	PresenceStatus model.PresenceStatus
	Email          string
	Phone          string
	Version        uint64
}

func (s *IdentityService) HouseholdSettings(ctx context.Context, actor AuthenticatedIdentity) (model.Household, error) {
	return s.repository.GetHouseholdByID(ctx, actor.HouseholdID)
}

func (s *IdentityService) UpdateHouseholdSettings(ctx context.Context, actor AuthenticatedIdentity, input UpdateHouseholdSettingsInput) (model.Household, error) {
	if err := requireOwner(actor); err != nil {
		return model.Household{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	if validateTextLength("household_name", input.Name, 1, 64) != nil ||
		len([]rune(input.Province)) > 64 || len([]rune(input.City)) > 64 {
		return model.Household{}, ErrInvalidInput
	}

	household, err := s.repository.UpdateHousehold(ctx, repository.UpdateHouseholdParams{
		ID:          actor.HouseholdID,
		DisplayName: input.Name,
		Province:    input.Province,
		City:        input.City,
		Version:     input.Version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return model.Household{}, ErrConflict
	}
	return household, err
}

func (s *IdentityService) Profile(ctx context.Context, actor AuthenticatedIdentity) (model.Member, error) {
	member, err := s.repository.GetMemberByID(ctx, actor.MemberID)
	if err != nil {
		return model.Member{}, err
	}
	if member.HouseholdID != actor.HouseholdID || member.Status != model.MemberStatusActive {
		return model.Member{}, ErrForbidden
	}
	return member, nil
}

func (s *IdentityService) UpdateProfile(ctx context.Context, actor AuthenticatedIdentity, input UpdateProfileInput) (model.Member, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	email, err := normalizeOptionalEmail(input.Email)
	if err != nil {
		return model.Member{}, err
	}
	phone, err := normalizeOptionalPhone(input.Phone)
	if err != nil {
		return model.Member{}, err
	}
	if validateTextLength("display_name", input.DisplayName, 1, 64) != nil ||
		!input.AvatarKey.Valid() ||
		(input.PresenceStatus != "" && !input.PresenceStatus.Valid()) {
		return model.Member{}, ErrInvalidInput
	}

	member, err := s.repository.UpdateMemberProfile(ctx, repository.UpdateMemberProfileParams{
		MemberID:       actor.MemberID,
		HouseholdID:    actor.HouseholdID,
		DisplayName:    input.DisplayName,
		AvatarKey:      input.AvatarKey,
		PresenceStatus: input.PresenceStatus,
		Email:          email,
		PhoneE164:      phone,
		Version:        input.Version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return model.Member{}, ErrConflict
	}
	return member, err
}

var e164PhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func normalizeOptionalEmail(value string) (*string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > 254 {
		return nil, fmt.Errorf("%w: email is too long", ErrInvalidInput)
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || address.Name != "" || !strings.Contains(value, "@") {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	return &value, nil
}

func normalizeOptionalPhone(value string) (*string, error) {
	value = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(value))
	if value == "" {
		return nil, nil
	}
	if len(value) == 11 && strings.HasPrefix(value, "1") {
		value = "+86" + value
	}
	if !e164PhonePattern.MatchString(value) {
		return nil, fmt.Errorf("%w: invalid phone number", ErrInvalidInput)
	}
	return &value, nil
}
