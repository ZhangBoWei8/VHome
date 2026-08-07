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
