package postgres

const (
	querySelectUserByWhatsApp = `
		SELECT id, whatsapp_number, display_name, email, is_admin, status, created_at, updated_at, deleted_at
		FROM users
		WHERE whatsapp_number = $1 AND deleted_at IS NULL
	`

	querySelectUserByID = `
		SELECT id, whatsapp_number, display_name, email, is_admin, status, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	queryInsertUser = `
		INSERT INTO users (id, whatsapp_number, display_name, email, is_admin, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, whatsapp_number, display_name, email, is_admin, status, created_at, updated_at, deleted_at
	`

	querySoftDeleteUser = `
		UPDATE users
		SET deleted_at = $1, status = 'DELETED', updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	queryDeactivateHistory = `
		UPDATE user_whatsapp_history
		SET active = false, unlinked_at = $1, reason = $2
		WHERE user_id = $3 AND active = true
	`

	queryDeactivateHistoryOnSoftDelete = `
		UPDATE user_whatsapp_history
		SET active = false, unlinked_at = $1, reason = 'user_soft_deleted'
		WHERE user_id = $2 AND active = true
	`

	queryInsertHistory = `
		INSERT INTO user_whatsapp_history (id, user_id, number, active, linked_at)
		VALUES ($1, $2, $3, true, $4)
	`

	queryUpdateUserWhatsApp = `
		UPDATE users
		SET whatsapp_number = $1, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`
)
