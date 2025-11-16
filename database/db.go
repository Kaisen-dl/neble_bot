package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
	statsUpdater func()
}

func New(connectionString string, statsUpdater func()) (*DB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Successfully connected to PostgreSQL")
	return &DB{db, statsUpdater}, nil
}

func (db *DB) AddUserRole(userID, userName, roleID, roleName string, expiresAt time.Time) error {
	query := `INSERT INTO user_roles (user_id, user_name, role_id, role_name, expires_at, message_id) 
              VALUES ($1, $2, $3, $4, $5, '')`
	result, err := db.Exec(query, userID, userName, roleID, roleName, expiresAt)
	if err != nil {
		log.Printf("Error inserting user role: %v", err)
		return err
	}

	rows, _ := result.RowsAffected()
	log.Printf("Successfully inserted role for user %s, role %s, affected rows: %d", userName, roleName, rows)

	if db.statsUpdater != nil {
		go db.statsUpdater()
	}
	return nil
}

func (db *DB) GetExpiredRoles() ([]UserRole, error) {
	query := `SELECT id, user_id, user_name, role_id, role_name, created_at, expires_at, is_active, renewal_status
              FROM user_roles 
              WHERE expires_at < NOW() AND is_active = true AND renewal_status = 'pending'`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []UserRole
	for rows.Next() {
		var role UserRole
		err := rows.Scan(&role.ID, &role.UserID, &role.UserName, &role.RoleID, &role.RoleName,
			&role.CreatedAt, &role.ExpiresAt, &role.IsActive, &role.RenewalStatus)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	log.Printf("Query returned %d expired roles", len(roles))
	return roles, nil
}

func (db *DB) GetRoleByID(id int) (*UserRole, error) {
	query := `SELECT id, user_id, user_name, role_id, role_name, created_at, expires_at, is_active, renewal_status
              FROM user_roles WHERE id = $1`

	var role UserRole
	err := db.QueryRow(query, id).Scan(
		&role.ID,
		&role.UserID,
		&role.UserName,
		&role.RoleID,
		&role.RoleName,
		&role.CreatedAt,
		&role.ExpiresAt,
		&role.IsActive,
		&role.RenewalStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role with ID %d not found", id)
		}
		return nil, err
	}

	return &role, nil
}

func (db *DB) UpdateRenewalStatus(id int, status string) error {
	validStatuses := map[string]bool{
		"pending":          true,
		"waiting_response": true,
		"confirmed":        true,
		"rejected":         true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	query := `UPDATE user_roles SET renewal_status = $1 WHERE id = $2`
	_, err := db.Exec(query, status, id)
	return err
}

func (db *DB) ExtendRole(id int, newExpiresAt time.Time) error {
	query := `UPDATE user_roles 
              SET expires_at = $1, is_active = true, renewal_status = 'pending' 
              WHERE id = $2`
	_, err := db.Exec(query, newExpiresAt, id)

	if db.statsUpdater != nil {
		go db.statsUpdater()
	}

	return err
}

func (db *DB) DeactivateRole(id int) error {
	query := `UPDATE user_roles 
              SET is_active = false, renewal_status = 'rejected' 
              WHERE id = $1`
	_, err := db.Exec(query, id)

	if db.statsUpdater != nil {
		go db.statsUpdater()
	}

	return err
}

func (db *DB) GetActiveRoleByUserID(userID string) (*UserRole, error) {
	query := `SELECT id, user_id, user_name, role_id, role_name, created_at, expires_at, is_active, renewal_status
              FROM user_roles WHERE user_id = $1 AND is_active = true`

	var role UserRole
	err := db.QueryRow(query, userID).Scan(
		&role.ID, &role.UserID, &role.UserName, &role.RoleID, &role.RoleName,
		&role.CreatedAt, &role.ExpiresAt, &role.IsActive, &role.RenewalStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Нет активной роли - это нормально
		}
		return nil, err
	}

	return &role, nil
}

func (db *DB) RemoveUserRole(userID string) error {
	query := `UPDATE user_roles 
              SET is_active = false, renewal_status = 'changed' 
              WHERE user_id = $1`
	result, err := db.Exec(query, userID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	log.Printf("Removed active role for user %s, affected rows: %d", userID, rows)
	return nil
}

func (db *DB) GetActiveRoleIDByUserID(userID string) (string, error) {
	query := `SELECT role_id FROM user_roles WHERE user_id = $1 AND is_active = true`

	var roleID string
	err := db.QueryRow(query, userID).Scan(&roleID)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no active role found for user %s", userID)
		}
		return "", err
	}

	return roleID, nil
}

// GetUserRole получает любую запись о пользователе (активную или нет)
func (db *DB) GetUserRole(userID string) (*UserRole, error) {
	query := `SELECT id, user_id, user_name, role_id, role_name, created_at, expires_at, is_active, renewal_status
              FROM user_roles WHERE user_id = $1
              ORDER BY created_at DESC LIMIT 1`

	var role UserRole
	err := db.QueryRow(query, userID).Scan(
		&role.ID, &role.UserID, &role.UserName, &role.RoleID, &role.RoleName,
		&role.CreatedAt, &role.ExpiresAt, &role.IsActive, &role.RenewalStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user %s not found", userID)
		}
		return nil, err
	}

	return &role, nil
}

// UpdateUserRole обновляет существующую запись пользователя
func (db *DB) UpdateUserRole(userID, roleID, roleName string, expiresAt time.Time) error {
	query := `UPDATE user_roles 
              SET role_id = $1, role_name = $2, expires_at = $3, 
                  is_active = true, renewal_status = 'pending', created_at = NOW(), message_id = ''
              WHERE user_id = $4`
	result, err := db.Exec(query, roleID, roleName, expiresAt, userID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	log.Printf("Updated role for user %s, affected rows: %d", userID, rows)

	if db.statsUpdater != nil {
		go db.statsUpdater()
	}

	return nil
}

func (db *DB) SetRenewalMessageID(roleID int, messageID string) error {
	query := `UPDATE user_roles SET message_id = $1 WHERE id = $2`
	_, err := db.Exec(query, messageID, roleID)
	return err
}

// Добавляем метод для получения ID сообщения
func (db *DB) GetRenewalMessageID(roleID int) (string, error) {
	query := `SELECT message_id FROM user_roles WHERE id = $1`
	var messageID string
	err := db.QueryRow(query, roleID).Scan(&messageID)
	if err != nil {
		return "", err
	}
	return messageID, nil
}
