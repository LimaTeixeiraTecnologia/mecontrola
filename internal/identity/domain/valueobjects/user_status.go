package valueobjects

type UserStatus uint8

const (
	UserStatusUnknown UserStatus = iota
	UserStatusActive
	UserStatusBlocked
	UserStatusDeleted
)

func (s UserStatus) String() string {
	switch s {
	case UserStatusActive:
		return "ACTIVE"
	case UserStatusBlocked:
		return "BLOCKED"
	case UserStatusDeleted:
		return "DELETED"
	default:
		return "UNKNOWN"
	}
}

func ParseUserStatus(s string) (UserStatus, bool) {
	switch s {
	case "ACTIVE":
		return UserStatusActive, true
	case "BLOCKED":
		return UserStatusBlocked, true
	case "DELETED":
		return UserStatusDeleted, true
	default:
		return UserStatusUnknown, false
	}
}
