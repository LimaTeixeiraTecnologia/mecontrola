package services

func DeriveClosingDay(dueDay, offsetDays int) int {
	d := ((dueDay-1-offsetDays)%31 + 31) % 31
	return d + 1
}
