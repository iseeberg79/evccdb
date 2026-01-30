package evccdb

import (
	"testing"
)

func TestOpen(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	if client.db == nil {
		t.Fatal("Database connection is nil")
	}
}

func TestGetTables(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	tables, err := client.GetTables()
	if err != nil {
		t.Fatalf("Failed to get tables: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("No tables found")
	}

	expectedTables := []string{"caches", "configs", "grid_sessions", "meters", "sessions", "settings"}
	for _, expected := range expectedTables {
		found := false
		for _, table := range tables {
			if table == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected table %s not found", expected)
		}
	}
}

func TestTableExists(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	exists, err := client.TableExists("settings")
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if !exists {
		t.Fatal("Expected settings table to exist")
	}

	exists, err = client.TableExists("nonexistent")
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}

	if exists {
		t.Fatal("Expected nonexistent table to not exist")
	}
}

func TestGetTableColumns(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	cols, err := client.GetTableColumns("settings")
	if err != nil {
		t.Fatalf("Failed to get table columns: %v", err)
	}

	if len(cols) == 0 {
		t.Fatal("No columns found for settings table")
	}

	expectedCols := map[string]bool{"key": true, "value": true}
	for _, col := range cols {
		if !expectedCols[col.Name] {
			t.Errorf("Unexpected column: %s", col.Name)
		}
		delete(expectedCols, col.Name)
	}

	if len(expectedCols) > 0 {
		for col := range expectedCols {
			t.Errorf("Expected column %s not found", col)
		}
	}
}

func TestGetRowCount(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	count, err := client.GetRowCount("settings")
	if err != nil {
		t.Fatalf("Failed to get row count: %v", err)
	}

	if count <= 0 {
		t.Fatal("Expected at least one row in settings table")
	}
}

func TestResolveTables(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	tests := []struct {
		name     string
		mode     TransferMode
		expected []string
	}{
		{
			name:     "Config mode",
			mode:     TransferConfig,
			expected: []string{"settings", "configs", "caches"},
		},
		{
			name:     "Metrics mode",
			mode:     TransferMetrics,
			expected: []string{"meters", "sessions", "grid_sessions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables, err := client.ResolveTables(TransferOptions{Mode: tt.mode})
			if err != nil {
				t.Fatalf("Failed to resolve tables: %v", err)
			}

			if len(tables) != len(tt.expected) {
				t.Errorf("Expected %d tables, got %d", len(tt.expected), len(tables))
			}

			for _, exp := range tt.expected {
				found := false
				for _, table := range tables {
					if table == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected table %s not found", exp)
				}
			}
		})
	}
}

func TestResolveTablesValidation(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	// Valid table names should work
	_, err := client.ResolveTables(TransferOptions{Tables: []string{"settings", "configs"}})
	if err != nil {
		t.Errorf("Valid table names should not error: %v", err)
	}

	// Invalid table name should error
	_, err = client.ResolveTables(TransferOptions{Tables: []string{"valid", "invalid;DROP TABLE"}})
	if err == nil {
		t.Error("Invalid table name should error")
	}
}

func TestClose(t *testing.T) {
	client, _ := createTestDB(t)

	err := client.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Calling Close again should not error
	err = client.Close()
	if err != nil {
		t.Fatalf("Second close should not error: %v", err)
	}
}

func TestCloseNilDatabase(t *testing.T) {
	client := &Client{}
	err := client.Close()
	if err != nil {
		t.Fatalf("Close on nil database should not error: %v", err)
	}
}
