package evccdb

import (
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/mattn/go-sqlite3"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier checks if a string is safe to use as a SQL identifier
func ValidateIdentifier(name string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q", name)
	}
	return nil
}

// Client represents a connection to an evcc SQLite database
type Client struct {
	db   *sql.DB
	path string
}

// Open opens a connection to an evcc SQLite database
func Open(path string) (*Client, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the database is accessible
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{
		db:   db,
		path: path,
	}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// GetTables returns a list of all tables in the database
func (c *Client) GetTables() ([]string, error) {
	rows, err := c.db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// TableExists checks if a table exists in the database
func (c *Client) TableExists(name string) (bool, error) {
	var count int
	err := c.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name = ?
	`, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %w", err)
	}
	return count > 0, nil
}

// ColumnInfo represents information about a column
type ColumnInfo struct {
	Name    string
	Type    string
	NotNull bool
	Default *string
	Primary bool
}

// GetTableColumns returns the columns for a table
func (c *Client) GetTableColumns(table string) ([]ColumnInfo, error) {
	rows, err := c.db.Query(fmt.Sprintf("PRAGMA table_info(`%s`)", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query columns for %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var columns []ColumnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue *string
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}

		columns = append(columns, ColumnInfo{
			Name:    name,
			Type:    colType,
			NotNull: notNull != 0,
			Default: dfltValue,
			Primary: pk != 0,
		})
	}

	return columns, rows.Err()
}

// GetRowCount returns the number of rows in a table
func (c *Client) GetRowCount(table string) (int, error) {
	var count int
	err := c.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows in %s: %w", table, err)
	}
	return count, nil
}

// GetConfigTables returns the list of configuration tables
func (c *Client) GetConfigTables() []string {
	return []string{"settings", "configs", "caches"}
}

// GetMetricsTables returns the list of metrics tables
func (c *Client) GetMetricsTables() []string {
	return []string{"meters", "sessions", "grid_sessions"}
}

// GetAllTables returns all known tables
func (c *Client) GetAllTables() []string {
	return append(c.GetConfigTables(), c.GetMetricsTables()...)
}

// ResolveTables returns the list of tables based on the transfer mode
func (c *Client) ResolveTables(opts TransferOptions) ([]string, error) {
	if len(opts.Tables) > 0 {
		for _, t := range opts.Tables {
			if err := ValidateIdentifier(t); err != nil {
				return nil, err
			}
		}
		return opts.Tables, nil
	}

	switch opts.Mode {
	case TransferConfig:
		return c.GetConfigTables(), nil
	case TransferMetrics:
		return c.GetMetricsTables(), nil
	case TransferAll:
		return c.GetAllTables(), nil
	default:
		return nil, fmt.Errorf("unknown transfer mode: %d", opts.Mode)
	}
}
