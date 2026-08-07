package model

import (
	"net"
	"time"
)

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "OWNER"
	MemberRoleAdmin  MemberRole = "ADMIN"
	MemberRoleMember MemberRole = "MEMBER"
)

type SessionRevokeReason string

const (
	SessionRevokeReasonLogout          SessionRevokeReason = "LOGOUT"
	SessionRevokeReasonMemberDisabled  SessionRevokeReason = "MEMBER_DISABLED"
	SessionRevokeReasonRoleChanged     SessionRevokeReason = "ROLE_CHANGED"
	SessionRevokeReasonPasswordChanged SessionRevokeReason = "PASSWORD_CHANGED"
	SessionRevokeReasonAdminRevoked    SessionRevokeReason = "ADMIN_REVOKED"
)

func (r MemberRole) Valid() bool {
	switch r {
	case MemberRoleOwner, MemberRoleAdmin, MemberRoleMember:
		return true
	default:
		return false
	}
}

type MemberStatus string

type PresenceStatus string

type MemberAvatar string

const (
	MemberAvatarInitials MemberAvatar = "initials"
	MemberAvatarMan      MemberAvatar = "man"
	MemberAvatarWoman    MemberAvatar = "woman"
	MemberAvatarBoy      MemberAvatar = "boy"
	MemberAvatarGirl     MemberAvatar = "girl"
	MemberAvatarDog      MemberAvatar = "dog"
)

func (a MemberAvatar) Valid() bool {
	switch a {
	case MemberAvatarInitials, MemberAvatarMan, MemberAvatarWoman,
		MemberAvatarBoy, MemberAvatarGirl, MemberAvatarDog:
		return true
	default:
		return false
	}
}

const (
	PresenceHome     PresenceStatus = "HOME"
	PresenceSchool   PresenceStatus = "SCHOOL"
	PresenceWorking  PresenceStatus = "WORKING"
	PresenceOut      PresenceStatus = "OUT"
	PresenceNapping  PresenceStatus = "NAPPING"
	PresenceResting  PresenceStatus = "RESTING"
	PresenceSick     PresenceStatus = "SICK"
	PresenceStudying PresenceStatus = "STUDYING"
)

func (s PresenceStatus) Valid() bool {
	switch s {
	case PresenceHome, PresenceSchool, PresenceWorking, PresenceOut,
		PresenceNapping, PresenceResting, PresenceSick, PresenceStudying:
		return true
	default:
		return false
	}
}

const (
	MemberStatusPending  MemberStatus = "PENDING"
	MemberStatusActive   MemberStatus = "ACTIVE"
	MemberStatusRejected MemberStatus = "REJECTED"
	MemberStatusDisabled MemberStatus = "DISABLED"
)

func (s MemberStatus) Valid() bool {
	switch s {
	case
		MemberStatusPending,
		MemberStatusActive,
		MemberStatusRejected,
		MemberStatusDisabled:
		return true
	default:
		return false
	}
}

type Household struct {
	ID                  uint64
	LoginName           string
	DisplayName         string
	JoinSecretHash      string
	RegistrationEnabled bool
	AvatarType          string
	AvatarValue         string
	Province            string
	City                string
	Version             uint64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Member struct {
	ID                 uint64
	HouseholdID        uint64
	Username           string
	UsernameNormalized string
	DisplayName        string
	PasswordHash       string
	Role               MemberRole
	Status             MemberStatus
	ReviewedBy         *uint64
	ReviewedAt         *time.Time
	LastLoginAt        *time.Time
	PresenceStatus     PresenceStatus
	AvatarKey          MemberAvatar
	Version            uint64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Session struct {
	ID            uint64
	HouseholdID   uint64
	MemberID      uint64
	TokenHash     []byte
	CSRFTokenHash []byte
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	CreatedIP     net.IP
	UserAgent     string
	RevokedAt     *time.Time
	RevokeReason  SessionRevokeReason
	CreatedAt     time.Time
}
