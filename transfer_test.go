package evccdb

import (
	"context"
	"database/sql"
	"testing"
)

func TestTransferConfigTables(t *testing.T) {
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Clear destination
	dst.db.Exec("DELETE FROM settings")
	dst.db.Exec("DELETE FROM configs")
	dst.db.Exec("DELETE FROM caches")

	srcSettingsCount, _ := src.GetRowCount("settings")

	ctx := context.Background()
	opts := TransferOptions{Mode: TransferConfig}

	err := Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	dstSettingsCount, _ := dst.GetRowCount("settings")
	if dstSettingsCount != srcSettingsCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcSettingsCount, dstSettingsCount)
	}
}

func TestTransferMetricsTables(t *testing.T) {
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Clear destination
	dst.db.Exec("DELETE FROM sessions")
	dst.db.Exec("DELETE FROM meters")

	srcSessionsCount, _ := src.GetRowCount("sessions")

	ctx := context.Background()
	opts := TransferOptions{Mode: TransferMetrics}

	err := Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	dstSessionsCount, _ := dst.GetRowCount("sessions")
	if dstSessionsCount != srcSessionsCount {
		t.Errorf("Sessions count mismatch: expected %d, got %d", srcSessionsCount, dstSessionsCount)
	}
}

func TestTransferWithExtraColumnInDest(t *testing.T) {
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Add extra column and clear data
	dst.db.Exec("ALTER TABLE settings ADD COLUMN extra TEXT DEFAULT 'test_value'")
	dst.db.Exec("DELETE FROM settings")

	srcCount, _ := src.GetRowCount("settings")

	ctx := context.Background()
	opts := TransferOptions{Mode: TransferConfig}

	err := Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	dstCount, _ := dst.GetRowCount("settings")
	if dstCount != srcCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcCount, dstCount)
	}

	// Check extra column has default value
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
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Clear destination
	dst.db.Exec("DELETE FROM settings")

	dstCountBefore, _ := dst.GetRowCount("settings")

	ctx := context.Background()
	opts := TransferOptions{
		Mode:   TransferConfig,
		DryRun: true,
	}

	err := Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Dry run transfer failed: %v", err)
	}

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
