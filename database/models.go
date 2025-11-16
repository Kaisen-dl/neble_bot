package database

import "time"

type UserRole struct {
	ID            int       `db:"id"`
	UserID        string    `db:"user_id"`
	UserName      string    `db:"user_name"`
	RoleID        string    `db:"role_id"`
	RoleName      string    `db:"role_name"`
	CreatedAt     time.Time `db:"created_at"`
	ExpiresAt     time.Time `db:"expires_at"`
	IsActive      bool      `db:"is_active"`
	RenewalStatus string    `db:"renewal_status"` // "pending", "waiting_response", "confirmed", "rejected"
	MessageID     string    `db:"message_id"`
}
