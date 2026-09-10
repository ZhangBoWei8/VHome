package service

import (
	"context"
	"errors"
	"fmt"

	"vhome/internal/model"
	"vhome/internal/repository"
)

func requireOwner(actor AuthenticatedIdentity) error {
	if actor.Role != model.MemberRoleOwner {
		return ErrForbidden
	}
	return nil
}

func (s *IdentityService) ListMembers(ctx context.Context, actor AuthenticatedIdentity) ([]model.Member, error) {
	if err := requireOwner(actor); err != nil {
		return nil, err
	}
	return s.repository.ListMembers(ctx, actor.HouseholdID)
}

// RosterMember is the household roster entry every member is allowed to see.
// It deliberately omits the credential and audit columns carried by
// model.Member so that callers cannot leak them.
type RosterMember struct {
	ID          uint64               `json:"id"`
	DisplayName string               `json:"display_name"`
	Username    string               `json:"username"`
	Role        model.MemberRole     `json:"role"`
	Presence    model.PresenceStatus `json:"presence_status"`
}

// HouseholdRoster lists the active members of the caller's household. Unlike
// ListMembers it is not restricted to the owner: knowing who lives in the
// household is ordinary information for everyone in it.
func (s *IdentityService) HouseholdRoster(ctx context.Context, actor AuthenticatedIdentity) ([]RosterMember, error) {
	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return nil, ErrUnauthenticated
	}

	members, err := s.repository.ListMembers(ctx, actor.HouseholdID)
	if err != nil {
		return nil, fmt.Errorf("list household roster: %w", err)
	}

	roster := make([]RosterMember, 0, len(members))
	for _, member := range members {
		if member.Status != model.MemberStatusActive {
			continue
		}

		roster = append(roster, RosterMember{
			ID:          member.ID,
			DisplayName: member.DisplayName,
			Username:    member.Username,
			Role:        member.Role,
			Presence:    member.PresenceStatus,
		})
	}

	return roster, nil
}
func (s *IdentityService) ReviewMember(ctx context.Context, actor AuthenticatedIdentity, id, version uint64, approve bool) (model.Member, error) {
	if err := requireOwner(actor); err != nil {
		return model.Member{}, err
	}
	p := repository.ReviewMemberParams{MemberID: id, HouseholdID: actor.HouseholdID, ReviewerID: actor.MemberID, Version: version}
	var v model.Member
	var e error
	if approve {
		v, e = s.repository.ApproveMember(ctx, p)
	} else {
		v, e = s.repository.RejectMember(ctx, p)
	}
	if errors.Is(e, repository.ErrConflict) {
		return v, ErrConflict
	}
	return v, e
}
func (s *IdentityService) ChangeMemberRole(ctx context.Context, actor AuthenticatedIdentity, id, version uint64, role model.MemberRole) (model.Member, error) {
	if err := requireOwner(actor); err != nil {
		return model.Member{}, err
	}
	if id == actor.MemberID {
		return model.Member{}, ErrForbidden
	}
	v, e := s.repository.UpdateMemberRole(ctx, repository.UpdateMemberRoleParams{MemberID: id, HouseholdID: actor.HouseholdID, Role: role, Version: version})
	if errors.Is(e, repository.ErrConflict) {
		return v, ErrConflict
	}
	if e != nil {
		return v, e
	}
	_, e = s.repository.RevokeMemberSessions(ctx, id, model.SessionRevokeReasonRoleChanged)
	return v, e
}
func (s *IdentityService) DisableMember(ctx context.Context, actor AuthenticatedIdentity, id, version uint64) (model.Member, error) {
	if err := requireOwner(actor); err != nil {
		return model.Member{}, err
	}
	if id == actor.MemberID {
		return model.Member{}, ErrForbidden
	}
	v, e := s.repository.DisableMember(ctx, repository.DisableMemberParams{MemberID: id, HouseholdID: actor.HouseholdID, Version: version})
	if errors.Is(e, repository.ErrConflict) {
		return v, ErrConflict
	}
	if e != nil {
		return v, e
	}
	_, e = s.repository.RevokeMemberSessions(ctx, id, model.SessionRevokeReasonMemberDisabled)
	if e != nil {
		return v, fmt.Errorf("revoke disabled member sessions: %w", e)
	}
	return v, nil
}
