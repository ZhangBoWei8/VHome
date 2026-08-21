package handler

import (
	"time"

	"vhome/internal/model"
	"vhome/internal/service"
)

type SetupRequest struct {
	Household struct {
		Name     string `json:"name" binding:"required,max=64"`
		Password string `json:"password" binding:"required,min=10,max=128"`
		Avatar   string `json:"avatar" binding:"omitempty,max=64"`
	} `json:"household" binding:"required"`

	Owner struct {
		Name     string `json:"name" binding:"required,max=64"`
		Password string `json:"password" binding:"required,min=10,max=128"`
	} `json:"owner" binding:"required"`
}

type RegisterMemberRequest struct {
	Name              string `json:"name" binding:"required,max=64"`
	Password          string `json:"password" binding:"required,min=10,max=128"`
	HouseholdPassword string `json:"household_password" binding:"required,min=10,max=128"`
}

type LoginRequest struct {
	Name     string `json:"name" binding:"required,max=64"`
	Password string `json:"password" binding:"required,max=128"`
}

type IconData struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type HouseholdData struct {
	ID       uint64   `json:"id"`
	Name     string   `json:"name"`
	Icon     IconData `json:"icon"`
	Province string   `json:"province"`
	City     string   `json:"city"`
	Version  uint64   `json:"version"`
}

type MemberData struct {
	ID             uint64               `json:"id"`
	Username       string               `json:"username"`
	DisplayName    string               `json:"display_name"`
	Role           model.MemberRole     `json:"role"`
	Status         model.MemberStatus   `json:"status"`
	PresenceStatus model.PresenceStatus `json:"presence_status"`
	AvatarKey      model.MemberAvatar   `json:"avatar_key"`
	Email          *string              `json:"email"`
	PhoneE164      *string              `json:"phone_e164"`
	Version        uint64               `json:"version"`
}

type SessionData struct {
	CSRFToken string `json:"csrf_token,omitempty"`

	ExpiresAt time.Time `json:"expires_at"`

	Household HouseholdData `json:"household"`
	Member    MemberData    `json:"member"`
}

type RegistrationData struct {
	Member MemberData `json:"member"`
}

func newHouseholdData(household model.Household) HouseholdData {
	return HouseholdData{
		ID:   household.ID,
		Name: household.DisplayName,

		Icon: IconData{
			Type:  household.AvatarType,
			Value: household.AvatarValue,
		},
		Province: household.Province,
		City:     household.City,

		Version: household.Version,
	}
}

func newMemberData(member model.Member) MemberData {
	return MemberData{
		ID:             member.ID,
		Username:       member.Username,
		DisplayName:    member.DisplayName,
		Role:           member.Role,
		Status:         member.Status,
		PresenceStatus: member.PresenceStatus,
		AvatarKey:      member.AvatarKey,
		Email:          member.Email,
		PhoneE164:      member.PhoneE164,
		Version:        member.Version,
	}
}

func newSetupSessionData(result service.SetupResult) SessionData {
	return SessionData{
		CSRFToken: result.CSRFToken,
		ExpiresAt: result.SessionExpiresAt,

		Household: newHouseholdData(
			result.Household,
		),

		Member: newMemberData(result.Owner),
	}
}

func newLoginSessionData(result service.LoginResult) SessionData {
	return SessionData{
		CSRFToken: result.CSRFToken,
		ExpiresAt: result.SessionExpiresAt,

		Household: newHouseholdData(
			result.Household,
		),

		Member: newMemberData(result.Member),
	}
}

func newCurrentSessionData(identity service.AuthenticatedIdentity) SessionData {
	return SessionData{
		ExpiresAt: identity.SessionExpiresAt,

		Household: HouseholdData{
			ID:   identity.HouseholdID,
			Name: identity.HouseholdName,

			Icon: IconData{
				Type:  identity.HouseholdAvatarType,
				Value: identity.HouseholdAvatarValue,
			},
			Province: identity.HouseholdProvince,
			City:     identity.HouseholdCity,

			Version: identity.HouseholdVersion,
		},

		Member: MemberData{
			ID:             identity.MemberID,
			Username:       identity.Username,
			DisplayName:    identity.DisplayName,
			Role:           identity.Role,
			Status:         model.MemberStatusActive,
			PresenceStatus: identity.PresenceStatus,
			AvatarKey:      identity.AvatarKey,
			Email:          identity.Email,
			PhoneE164:      identity.PhoneE164,
			Version:        identity.MemberVersion,
		},
	}
}
