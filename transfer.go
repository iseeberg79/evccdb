package evccdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Transfer transfers data from source to destination database based on options
func Transfer(ctx context.Context, src, dst *Client, opts TransferOptions) error {
	tables, err := src.ResolveTables(opts)
	if err != nil {
		return fmt.Errorf("failed to resolve tables: %w", err)
	}

	if opts.DryRun {
		fmt.Printf("DRY RUN: Would transfer %d tables\n", len(tables))
		for _, table := range tables {
			exists, err := dst.TableExists(table)
			if err != nil {
				return err
			}
			if !exists {
				fmt.Printf("  WARNING: Table %s does not exist in destination\n", table)
				continue
			}

			count, err := src.GetRowCount(table)
			if err != nil {
				return err
			}
			fmt.Printf("  %s: %d rows\n", table, count)
		}

		// Show rename previews
		for _, rename := range opts.LoadpointRenames {
			result, err := src.RenameLoadpointDryRun(ctx, rename.OldName, rename.NewName)
			if err != nil {
				return err
			}
			fmt.Printf("  Loadpoint rename %q -> %q: sessions=%d, settings=%d, configs=%d\n",
				rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
		}

		for _, rename := range opts.VehicleRenames {
			result, err := src.RenameVehicleDryRun(ctx, rename.OldName, rename.NewName)
			if err != nil {
				return err
			}
			fmt.Printf("  Vehicle rename %q -> %q: sessions=%d, settings=%d, configs=%d\n",
				rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
		}

		return nil
	}

	// Start a transaction on destination
	tx, err := dst.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range tables {
		exists, err := dst.TableExists(table)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Printf("WARNING: Table %s does not exist in destination, skipping\n", table)
			continue
		}

		count, err := copyTableWithTx(ctx, tx, src, dst, table)
		if err != nil {
			return fmt.Errorf("failed to copy table %s: %w", table, err)
		}

		if opts.OnProgress != nil {
			opts.OnProgress(table, count)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Apply renames after transfer completes
	for _, rename := range opts.LoadpointRenames {
		if _, err := dst.RenameLoadpoint(ctx, rename.OldName, rename.NewName); err != nil {
			return fmt.Errorf("failed to rename loadpoint %q to %q: %w", rename.OldName, rename.NewName, err)
		}
	}

	for _, rename := range opts.VehicleRenames {
		if _, err := dst.RenameVehicle(ctx, rename.OldName, rename.NewName); err != nil {
			return fmt.Errorf("failed to rename vehicle %q to %q: %w", rename.OldName, rename.NewName, err)
		}
	}

	return nil
}

// copyTableWithTx copies a table using a destination transaction
func copyTableWithTx(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, src, dst *Client, table string) (int, error) {
	// Get column information from both databases
	srcCols, err := src.GetTableColumns(table)
	if err != nil {
		return 0, err
	}

	dstCols, err := dst.GetTableColumns(table)
	if err != nil {
		return 0, err
	}

	// Find common columns
	commonCols := intersectColumns(srcCols, dstCols)
	if len(commonCols) == 0 {
		return 0, fmt.Errorf("no common columns found between source and destination for table %s", table)
	}

	// Check for columns in source that are missing in destination
	srcColMap := make(map[string]bool)
	for _, col := range srcCols {
		srcColMap[col.Name] = true
	}
	dstColMap := make(map[string]bool)
	for _, col := range dstCols {
		dstColMap[col.Name] = true
	}

	for _, col := range srcCols {
		if !dstColMap[col.Name] {
			fmt.Printf("WARNING: Column %s.%s exists in source but not in destination, will be skipped\n", table, col.Name)
		}
	}

	// Get row count first
	count, err := src.GetRowCount(table)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	// Build column names and copy rows using raw SQL from source
	colNames := make([]string, len(commonCols))
	colNameList := make([]string, len(commonCols))
	for i, col := range commonCols {
		colNames[i] = col.Name
		colNameList[i] = fmt.Sprintf("`%s`", col.Name)
	}

	// Get all data from source and copy to destination
	srcRows, err := src.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(colNameList, ", "), table))
	if err != nil {
		return 0, fmt.Errorf("failed to query source data: %w", err)
	}
	defer func() { _ = srcRows.Close() }()

	copied := 0
	for srcRows.Next() {
		values := make([]any, len(colNames))
		scanPtrs := make([]any, len(colNames))
		for i := range colNames {
			scanPtrs[i] = &values[i]
		}

		if err := srcRows.Scan(scanPtrs...); err != nil {
			return copied, fmt.Errorf("failed to scan row: %w", err)
		}

		// Build INSERT statement
		placeholders := make([]string, len(colNames))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		insertSQL := fmt.Sprintf("INSERT OR REPLACE INTO `%s` (%s) VALUES (%s)",
			table, strings.Join(colNameList, ", "), strings.Join(placeholders, ", "))

		_, err := tx.ExecContext(ctx, insertSQL, values...)
		if err != nil {
			return copied, fmt.Errorf("failed to insert row: %w", err)
		}

		copied++
	}

	return copied, srcRows.Err()
}

// intersectColumns finds the intersection of columns by name
func intersectColumns(src, dst []ColumnInfo) []ColumnInfo {
	dstMap := make(map[string]ColumnInfo)
	for _, col := range dst {
		dstMap[col.Name] = col
	}

	var result []ColumnInfo
	for _, col := range src {
		if _, exists := dstMap[col.Name]; exists {
			result = append(result, col)
		}
	}
	return result
}

// CopyTablesTo copies specific tables from source to destination
func (c *Client) CopyTablesTo(ctx context.Context, dst *Client, tables []string) error {
	tx, err := dst.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range tables {
		_, err := copyTableWithTx(ctx, tx, c, dst, table)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
