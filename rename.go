package evccdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// RenameResult contains the counts of renamed rows per table
type RenameResult struct {
	Sessions int
	Settings int
	Configs  int
}

// RenameLoadpoint updates a loadpoint name across all tables
func (c *Client) RenameLoadpoint(ctx context.Context, oldName, newName string) (RenameResult, error) {
	var result RenameResult

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Rename in sessions table
	count, err := c.renameInSessions(ctx, tx, "loadpoint", oldName, newName)
	if err != nil {
		return result, fmt.Errorf("failed to rename loadpoint in sessions: %w", err)
	}
	result.Sessions = count

	// 2. Rename in settings (lp<n>.title values)
	count, err = c.renameSettingsValue(ctx, tx, "lp%.title", oldName, newName)
	if err != nil {
		return result, fmt.Errorf("failed to rename loadpoint in settings: %w", err)
	}
	result.Settings = count

	// 3. Rename in configs JSON (class 5 = loadpoints)
	count, err = c.renameInConfigsJSON(ctx, tx, 5, oldName, newName)
	if err != nil {
		return result, fmt.Errorf("failed to rename loadpoint in configs: %w", err)
	}
	result.Configs = count

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// RenameVehicle updates a vehicle name across all tables
func (c *Client) RenameVehicle(ctx context.Context, oldName, newName string) (RenameResult, error) {
	var result RenameResult

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Rename in sessions table
	count, err := c.renameInSessions(ctx, tx, "vehicle", oldName, newName)
	if err != nil {
		return result, fmt.Errorf("failed to rename vehicle in sessions: %w", err)
	}
	result.Sessions = count

	// 2. Rename vehicle settings keys (vehicle.OldName.* -> vehicle.NewName.*)
	oldPrefix := "vehicle." + oldName + "."
	newPrefix := "vehicle." + newName + "."
	count, err = c.renameSettingsKeys(ctx, tx, oldPrefix, newPrefix)
	if err != nil {
		return result, fmt.Errorf("failed to rename vehicle settings keys: %w", err)
	}
	result.Settings = count

	// 3. Rename in configs JSON/YAML (class 3 = vehicles)
	count, err = c.renameInConfigsJSON(ctx, tx, 3, oldName, newName)
	if err != nil {
		return result, fmt.Errorf("failed to rename vehicle in configs: %w", err)
	}
	result.Configs = count

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// renameInSessions updates a column value in the sessions table
func (c *Client) renameInSessions(ctx context.Context, tx *sql.Tx, column, oldName, newName string) (int, error) {
	result, err := tx.ExecContext(ctx,
		fmt.Sprintf("UPDATE sessions SET `%s` = ? WHERE `%s` = ?", column, column),
		newName, oldName)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

// renameSettingsValue updates settings value where key matches pattern and value matches oldName
func (c *Client) renameSettingsValue(ctx context.Context, tx *sql.Tx, keyPattern, oldValue, newValue string) (int, error) {
	result, err := tx.ExecContext(ctx,
		"UPDATE settings SET value = ? WHERE key LIKE ? AND value = ?",
		newValue, keyPattern, oldValue)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

// renameSettingsKeys renames settings keys by replacing prefix
func (c *Client) renameSettingsKeys(ctx context.Context, tx *sql.Tx, oldPrefix, newPrefix string) (int, error) {
	// First, get all keys matching the old prefix
	rows, err := tx.QueryContext(ctx, "SELECT key, value FROM settings WHERE key LIKE ?", oldPrefix+"%")
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	type keyValue struct {
		key   string
		value string
	}
	var kvs []keyValue
	for rows.Next() {
		var kv keyValue
		if err := rows.Scan(&kv.key, &kv.value); err != nil {
			return 0, err
		}
		kvs = append(kvs, kv)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Insert with new prefix and delete old
	for _, kv := range kvs {
		newKey := newPrefix + strings.TrimPrefix(kv.key, oldPrefix)

		// Insert or replace with new key
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", newKey, kv.value)
		if err != nil {
			return 0, err
		}

		// Delete old key
		_, err = tx.ExecContext(ctx, "DELETE FROM settings WHERE key = ?", kv.key)
		if err != nil {
			return 0, err
		}
	}

	return len(kvs), nil
}

// renameInConfigsJSON updates title field in configs JSON for specified class
func (c *Client) renameInConfigsJSON(ctx context.Context, tx *sql.Tx, class int, oldTitle, newTitle string) (int, error) {
	// Query configs for the specified class
	rows, err := tx.QueryContext(ctx, "SELECT id, value FROM configs WHERE class = ?", class)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	type configRow struct {
		id    int
		value string
	}
	var configs []configRow
	for rows.Next() {
		var cfg configRow
		if err := rows.Scan(&cfg.id, &cfg.value); err != nil {
			return 0, err
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	updated := 0
	for _, cfg := range configs {
		// Try to parse as JSON
		var data map[string]any
		if err := json.Unmarshal([]byte(cfg.value), &data); err != nil {
			// Not JSON, try YAML-style title extraction
			if strings.Contains(cfg.value, "title: "+oldTitle) {
				newValue := strings.Replace(cfg.value, "title: "+oldTitle, "title: "+newTitle, 1)
				_, err := tx.ExecContext(ctx, "UPDATE configs SET value = ? WHERE id = ?", newValue, cfg.id)
				if err != nil {
					return updated, err
				}
				updated++
			}
			continue
		}

		// Check if title matches
		title, ok := data["title"].(string)
		if !ok || title != oldTitle {
			continue
		}

		// Update title
		data["title"] = newTitle
		newJSON, err := json.Marshal(data)
		if err != nil {
			return updated, err
		}

		_, err = tx.ExecContext(ctx, "UPDATE configs SET value = ? WHERE id = ?", string(newJSON), cfg.id)
		if err != nil {
			return updated, err
		}
		updated++
	}

	return updated, nil
}

// RenameLoadpointDryRun returns the counts of what would be renamed without making changes
func (c *Client) RenameLoadpointDryRun(ctx context.Context, oldName, newName string) (RenameResult, error) {
	var result RenameResult

	// Count sessions
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = ?", oldName).Scan(&count)
	if err != nil {
		return result, err
	}
	result.Sessions = count

	// Count settings
	err = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'lp%.title' AND value = ?", oldName).Scan(&count)
	if err != nil {
		return result, err
	}
	result.Settings = count

	// Count configs
	count, err = c.countConfigsWithTitle(ctx, 5, oldName)
	if err != nil {
		return result, err
	}
	result.Configs = count

	return result, nil
}

// RenameVehicleDryRun returns the counts of what would be renamed without making changes
func (c *Client) RenameVehicleDryRun(ctx context.Context, oldName, newName string) (RenameResult, error) {
	var result RenameResult

	// Count sessions
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = ?", oldName).Scan(&count)
	if err != nil {
		return result, err
	}
	result.Sessions = count

	// Count settings keys
	oldPrefix := "vehicle." + oldName + ".%"
	err = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE ?", oldPrefix).Scan(&count)
	if err != nil {
		return result, err
	}
	result.Settings = count

	// Count configs
	count, err = c.countConfigsWithTitle(ctx, 3, oldName)
	if err != nil {
		return result, err
	}
	result.Configs = count

	return result, nil
}

// countConfigsWithTitle counts configs in a class with matching title
func (c *Client) countConfigsWithTitle(ctx context.Context, class int, title string) (int, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT value FROM configs WHERE class = ?", class)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return 0, err
		}

		// Try JSON
		var data map[string]any
		if err := json.Unmarshal([]byte(value), &data); err != nil {
			// Try YAML-style
			if strings.Contains(value, "title: "+title) {
				count++
			}
			continue
		}

		if t, ok := data["title"].(string); ok && t == title {
			count++
		}
	}

	return count, rows.Err()
}

// DeleteLoadpointSessions deletes all sessions for a specific loadpoint
func (c *Client) DeleteLoadpointSessions(ctx context.Context, loadpoint string) (int, error) {
	result, err := c.db.ExecContext(ctx, "DELETE FROM sessions WHERE loadpoint = ?", loadpoint)
	if err != nil {
		return 0, fmt.Errorf("failed to delete sessions: %w", err)
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

// DeleteVehicleSessions deletes all sessions for a specific vehicle
func (c *Client) DeleteVehicleSessions(ctx context.Context, vehicle string) (int, error) {
	result, err := c.db.ExecContext(ctx, "DELETE FROM sessions WHERE vehicle = ?", vehicle)
	if err != nil {
		return 0, fmt.Errorf("failed to delete sessions: %w", err)
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

// CountLoadpointSessions counts sessions for a specific loadpoint
func (c *Client) CountLoadpointSessions(ctx context.Context, loadpoint string) (int, error) {
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = ?", loadpoint).Scan(&count)
	return count, err
}

// CountVehicleSessions counts sessions for a specific vehicle
func (c *Client) CountVehicleSessions(ctx context.Context, vehicle string) (int, error) {
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = ?", vehicle).Scan(&count)
	return count, err
}
