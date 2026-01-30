package evccdb

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestTransferConfigTables(t *testing.T) {
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema from source
	initializeDB(t, tmpFile.Name(), src)

	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Get counts before transfer
	srcSettingsCount, _ := src.GetRowCount("settings")

	// Perform transfer
	ctx := context.Background()
	opts := TransferOptions{
		Mode: TransferConfig,
	}

	err = Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	// Verify data was transferred
	dstSettingsCount, _ := dst.GetRowCount("settings")
	if dstSettingsCount != srcSettingsCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcSettingsCount, dstSettingsCount)
	}
}

func TestTransferMetricsTables(t *testing.T) {
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema from source
	initializeDB(t, tmpFile.Name(), src)

	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Get counts before transfer
	srcMetersCount, _ := src.GetRowCount("meters")

	// Perform transfer
	ctx := context.Background()
	opts := TransferOptions{
		Mode: TransferMetrics,
	}

	err = Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	// Verify data was transferred
	dstMetersCount, _ := dst.GetRowCount("meters")
	if dstMetersCount != srcMetersCount {
		t.Errorf("Meters count mismatch: expected %d, got %d", srcMetersCount, dstMetersCount)
	}
}

func TestTransferWithExtraColumnInDest(t *testing.T) {
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema from source
	initializeDB(t, tmpFile.Name(), src)

	// Add extra column to settings table in destination
	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}

	_, err = dst.db.Exec("ALTER TABLE settings ADD COLUMN extra TEXT DEFAULT 'test_value'")
	if err != nil {
		t.Fatalf("Failed to add extra column: %v", err)
	}
	dst.Close()

	// Reopen destination
	dst, err = Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to reopen destination database: %v", err)
	}
	defer dst.Close()

	// Get count from source
	srcCount, _ := src.GetRowCount("settings")

	// Perform transfer
	ctx := context.Background()
	opts := TransferOptions{
		Mode: TransferConfig,
	}

	err = Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	// Verify data was transferred and extra column has default value
	dstCount, _ := dst.GetRowCount("settings")
	if dstCount != srcCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcCount, dstCount)
	}

	// Check that extra column was not modified
	var extra string
	err = dst.db.QueryRow("SELECT extra FROM settings LIMIT 1").Scan(&extra)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Failed to query extra column: %v", err)
	}
	if extra != "test_value" {
		t.Errorf("Extra column should have default value, got %s", extra)
	}
}

func TestTransferDryRun(t *testing.T) {
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema from source
	initializeDB(t, tmpFile.Name(), src)

	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Get count before dry run
	dstCountBefore, _ := dst.GetRowCount("settings")

	// Perform dry run transfer
	ctx := context.Background()
	opts := TransferOptions{
		Mode:   TransferConfig,
		DryRun: true,
	}

	err = Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Dry run transfer failed: %v", err)
	}

	// Verify no data was transferred
	dstCountAfter, _ := dst.GetRowCount("settings")
	if dstCountAfter != dstCountBefore {
		t.Errorf("Dry run should not transfer data: before %d, after %d", dstCountBefore, dstCountAfter)
	}
}

func TestIntersectColumns(t *testing.T) {
	tests := []struct {
		name     string
		src      []ColumnInfo
		dst      []ColumnInfo
		expected int
	}{
		{
			name: "Identical columns",
			src: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
			},
			dst: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
			},
			expected: 2,
		},
		{
			name: "Destination has extra column",
			src: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
			},
			dst: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
				{Name: "extra", Type: "text"},
			},
			expected: 2,
		},
		{
			name: "Source has extra column",
			src: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
				{Name: "extra", Type: "text"},
			},
			dst: []ColumnInfo{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intersectColumns(tt.src, tt.dst)
			if len(result) != tt.expected {
				t.Errorf("Expected %d columns, got %d", tt.expected, len(result))
			}
		})
	}
}

// Helper functions

func initializeDB(t *testing.T, path string, src *Client) {
	// Get schema from source
	tables, err := src.GetTables()
	if err != nil {
		t.Fatalf("Failed to get source tables: %v", err)
	}

	dst, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}

	for _, table := range tables {
		// Get CREATE TABLE statement
		var sql string
		err := src.db.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&sql)
		if err != nil {
			t.Fatalf("Failed to get schema for %s: %v", table, err)
		}

		// Create table in destination
		_, err = dst.db.Exec(sql)
		if err != nil {
			t.Fatalf("Failed to create table %s in destination: %v", table, err)
		}
	}

	// Get and recreate indexes
	rows, err := src.db.Query(
		"SELECT sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL",
	)
	if err != nil {
		t.Fatalf("Failed to get indexes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			t.Fatalf("Failed to scan index: %v", err)
		}

		_, err = dst.db.Exec(sql)
		if err != nil && !contains(err.Error(), "already exists") {
			t.Logf("Warning: Failed to create index: %v", err)
		}
	}

	dst.Close()
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
