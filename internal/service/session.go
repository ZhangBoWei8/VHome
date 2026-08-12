package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vhome/internal/auth"
	"vhome/internal/model"
	"vhome/internal/repository"
)

const maxOpaqueTokenLength = 128

type AuthenticatedIdentity struct {
	SessionID        uint64
	SessionExpiresAt time.Time

	HouseholdID          uint64
	HouseholdName        string
	HouseholdAvatarType  string
	HouseholdAvatarValue string
	HouseholdVersion     uint64
	HouseholdProvince    string
	HouseholdCity        string

	MemberID       uint64
	Username       string
	DisplayName    string
	Role           model.MemberRole
	MemberVersion  uint64
	PresenceStatus model.PresenceStatus
	AvatarKey      model.MemberAvatar

	csrfTokenHash []byte
}

func (s *IdentityService) AuthenticateSession(ctx context.Context, plaintextToken string) (AuthenticatedIdentity, error) {
	if plaintextToken == "" || len(plaintextToken) > maxOpaqueTokenLength {
		return AuthenticatedIdentity{},
			ErrUnauthenticated
	}

	tokenHash := auth.HashToken(plaintextToken)

	session, err :=
		s.repository.GetActiveSessionByTokenHash(
			ctx,
			tokenHash,
		)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return AuthenticatedIdentity{},
				ErrUnauthenticated
		}

		return AuthenticatedIdentity{}, fmt.Errorf(
			"get active session: %w",
			err,
		)
	}

	member, err := s.repository.GetMemberByID(
		ctx,
		session.MemberID,
	)
	if err != nil {
		return AuthenticatedIdentity{}, fmt.Errorf(
			"get session member: %w",
			err,
		)
	}

	if member.HouseholdID != session.HouseholdID {
		return AuthenticatedIdentity{}, errors.New(
			"session household does not match member household",
		)
	}

	if member.Status != model.MemberStatusActive {
		return AuthenticatedIdentity{},
			ErrUnauthenticated
	}

	if !member.Role.Valid() {
		return AuthenticatedIdentity{}, fmt.Errorf(
			"session member has invalid role: %q",
			member.Role,
		)
	}

	household, err := s.repository.GetHouseholdByID(
		ctx,
		session.HouseholdID,
	)
	if err != nil {
		return AuthenticatedIdentity{}, fmt.Errorf(
			"get session household: %w",
			err,
		)
	}

	if err := s.repository.TouchSession(
		ctx,
		session.ID,
	); err != nil {
		return AuthenticatedIdentity{}, fmt.Errorf(
			"touch authenticated session: %w",
			err,
		)
	}

	return AuthenticatedIdentity{
		SessionID:        session.ID,
		SessionExpiresAt: session.ExpiresAt,

		HouseholdID:          household.ID,
		HouseholdName:        household.DisplayName,
		HouseholdAvatarType:  household.AvatarType,
		HouseholdAvatarValue: household.AvatarValue,
		HouseholdVersion:     household.Version,
		HouseholdProvince:    household.Province,
		HouseholdCity:        household.City,

		MemberID:       member.ID,
		Username:       member.Username,
		DisplayName:    member.DisplayName,
		Role:           member.Role,
		MemberVersion:  member.Version,
		PresenceStatus: member.PresenceStatus,
		AvatarKey:      member.AvatarKey,

		csrfTokenHash: append(
			[]byte(nil),
			session.CSRFTokenHash...,
		),
	}, nil
}

func (s *IdentityService) VerifyCSRFToken(identity AuthenticatedIdentity, plaintextToken string) error {
	if plaintextToken == "" ||
		len(plaintextToken) > maxOpaqueTokenLength {
		return ErrInvalidCSRFToken
	}

	if !auth.VerifyToken(
		plaintextToken,
		identity.csrfTokenHash,
	) {
		return ErrInvalidCSRFToken
	}

	return nil
}

func (s *IdentityService) Logout(ctx context.Context, identity AuthenticatedIdentity, csrfToken string) error {
	if err := s.VerifyCSRFToken(identity, csrfToken); err != nil {
		return err
	}

	if err := s.repository.RevokeSession(ctx, identity.SessionID, model.SessionRevokeReasonLogout); err != nil {
		return fmt.Errorf(
			"logout session: %w",
			err,
		)
	}

	return nil
}
