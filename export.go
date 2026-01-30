package evccdb

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ExportJSON exports selected tables to JSON
func (c *Client) ExportJSON(w io.Writer, opts TransferOptions) error {
	tables, err := c.ResolveTables(opts)
	if err != nil {
		return fmt.Errorf("failed to resolve tables: %w", err)
	}

	data := make(map[string]any)

	for _, table := range tables {
		exists, err := c.TableExists(table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		rows, err := c.exportTable(table)
		if err != nil {
			return fmt.Errorf("failed to export table %s: %w", table, err)
		}
		data[table] = rows

		if opts.OnProgress != nil {
			opts.OnProgress(table, len(rows))
		}
	}

	export := ExportFormat{
		Version:    "1",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Tables:     data,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(export)
}

// exportTable exports a single table to a slice of maps
func (c *Client) exportTable(table string) ([]map[string]any, error) {
	rows, err := c.db.Query(fmt.Sprintf("SELECT * FROM `%s`", table))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		entry := make(map[string]any)
		for i, col := range columns {
			var v any
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		result = append(result, entry)
	}

	return result, rows.Err()
}

// getColumnTypesForTable gets the SQL types of columns
func (c *Client) getColumnTypesForTable(table string) (map[string]string, error) {
	cols, err := c.GetTableColumns(table)
	if err != nil {
		return nil, err
	}

	types := make(map[string]string)
	for _, col := range cols {
		types[col.Name] = col.Type
	}
	return types, nil
}

// formatValueForSQL formats a value for SQL insertion
func formatValueForSQL(val any, _ string) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case string:
		// Escape single quotes
		escaped := escapeSQL(v)
		return fmt.Sprintf("'%s'", escaped)
	case float64:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return "NULL"
	}
}

// escapeSQL escapes a string for SQL by doubling single quotes
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
