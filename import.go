package evccdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
)

// ImportJSON imports data from a JSON export file
func (c *Client) ImportJSON(r io.Reader, opts TransferOptions) error {
	var export ExportFormat
	if err := json.NewDecoder(r).Decode(&export); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	if export.Version != "1" {
		return fmt.Errorf("unsupported export format version: %s", export.Version)
	}

	ctx := context.Background()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Determine which tables to import
	var tablesToImport []string
	if len(opts.Tables) > 0 {
		tablesToImport = opts.Tables
	} else {
		switch opts.Mode {
		case TransferConfig:
			tablesToImport = c.GetConfigTables()
		case TransferMetrics:
			tablesToImport = c.GetMetricsTables()
		case TransferAll:
			for table := range export.Tables {
				tablesToImport = append(tablesToImport, table)
			}
		default:
			return fmt.Errorf("unknown transfer mode: %d", opts.Mode)
		}
	}

	for _, table := range tablesToImport {
		tableData, exists := export.Tables[table]
		if !exists {
			continue
		}

		rows, ok := tableData.([]any)
		if !ok {
			continue
		}

		count, err := c.importTableWithTx(ctx, tx, table, rows)
		if err != nil {
			return fmt.Errorf("failed to import table %s: %w", table, err)
		}

		if opts.OnProgress != nil {
			opts.OnProgress(table, count)
		}
	}

	return tx.Commit()
}

// importTableWithTx imports a table using a transaction
func (c *Client) importTableWithTx(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, table string, rows []any) (int, error) {
	// Get column types for the table
	columnTypes, err := c.getColumnTypesForTable(table)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, rowData := range rows {
		rowMap, ok := rowData.(map[string]any)
		if !ok {
			continue
		}

		// Filter columns to only those that exist in the table
		filteredRow := make(map[string]any)
		for key, val := range rowMap {
			if _, exists := columnTypes[key]; exists {
				filteredRow[key] = val
			}
		}

		if len(filteredRow) == 0 {
			continue
		}

		// Build and execute INSERT
		sql := buildInsertFromMapWithColumns(table, filteredRow, columnTypes)
		if _, err := tx.ExecContext(ctx, sql); err != nil {
			return 0, fmt.Errorf("failed to insert row: %w", err)
		}

		count++
	}

	return count, nil
}

// buildInsertFromMapWithColumns builds an INSERT statement from a row map
func buildInsertFromMapWithColumns(table string, row map[string]any, columnTypes map[string]string) string {
	var cols []string
	var vals []string

	for col, val := range row {
		cols = append(cols, fmt.Sprintf("`%s`", col))
		colType := columnTypes[col]
		vals = append(vals, formatValueForSQL(val, colType))
	}

	colsStr := "(" + cols[0]
	for _, col := range cols[1:] {
		colsStr += ", " + col
	}
	colsStr += ")"

	valsStr := "(" + vals[0]
	for _, val := range vals[1:] {
		valsStr += ", " + val
	}
	valsStr += ")"

	return fmt.Sprintf("INSERT OR REPLACE INTO `%s` %s VALUES %s", table, colsStr, valsStr)
}
