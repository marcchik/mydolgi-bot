package domain

import "time"

type User struct {
	ID         int64
	TelegramID int64
	Username   *string
	FirstName  *string
	LastName   *string
	CreatedAt  time.Time
}

type Debt struct {
	ID         int64
	CreditorID int64
	DebtorID   int64
	AmountCents int64
	Currency   string
	DueDate    time.Time // date-only semantics
	Status     string
	CreatedAt  time.Time
}
