package evccdb

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestExportJSON(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	var buf bytes.Buffer
	opts := TransferOptions{Mode: TransferConfig}

	err := client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	var export ExportFormat
	err = json.Unmarshal(buf.Bytes(), &export)
	if err != nil {
		t.Fatalf("Failed to unmarshal exported JSON: %v", err)
	}

	if export.Version != "1" {
		t.Errorf("Expected version 1, got %s", export.Version)
	}

	if export.ExportedAt == "" {
		t.Error("ExportedAt should not be empty")
	}

	configTables := []string{"settings", "configs", "caches"}
	for _, table := range configTables {
		if _, exists := export.Tables[table]; !exists {
			t.Errorf("Expected table %s in export", table)
		}
	}
}

func TestExportJSONMetrics(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	var buf bytes.Buffer
	opts := TransferOptions{Mode: TransferMetrics}

	err := client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	var export ExportFormat
	err = json.Unmarshal(buf.Bytes(), &export)
	if err != nil {
		t.Fatalf("Failed to unmarshal exported JSON: %v", err)
	}

	metricsTables := []string{"meters", "sessions", "grid_sessions"}
	for _, table := range metricsTables {
		if _, exists := export.Tables[table]; !exists {
			t.Errorf("Expected table %s in export", table)
		}
	}
}

func TestExportJSONAll(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	var buf bytes.Buffer
	opts := TransferOptions{Mode: TransferAll}

	err := client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	var export ExportFormat
	err = json.Unmarshal(buf.Bytes(), &export)
	if err != nil {
		t.Fatalf("Failed to unmarshal exported JSON: %v", err)
	}

	allTables := []string{"settings", "configs", "caches", "meters", "sessions", "grid_sessions"}
	for _, table := range allTables {
		if _, exists := export.Tables[table]; !exists {
			t.Errorf("Expected table %s in export", table)
		}
	}
}

func TestImportJSON(t *testing.T) {
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	// Export data
	var buf bytes.Buffer
	opts := TransferOptions{Mode: TransferConfig}

	err := src.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Clear destination
	dst.db.Exec("DELETE FROM settings")
	dst.db.Exec("DELETE FROM configs")

	srcCount, _ := src.GetRowCount("settings")

	// Import data
	importBuf := bytes.NewReader(buf.Bytes())
	err = dst.ImportJSON(importBuf, opts)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	dstCount, _ := dst.GetRowCount("settings")
	if dstCount != srcCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcCount, dstCount)
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	src, srcCleanup := createTestDB(t)
	defer srcCleanup()

	srcSettingsCount, _ := src.GetRowCount("settings")
	srcConfigsCount, _ := src.GetRowCount("configs")

	// Export
	var buf bytes.Buffer
	opts := TransferOptions{Mode: TransferConfig}

	err := src.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	dst, dstCleanup := createTestDB(t)
	defer dstCleanup()

	// Clear destination
	dst.db.Exec("DELETE FROM settings")
	dst.db.Exec("DELETE FROM configs")

	// Import
	importBuf := bytes.NewReader(buf.Bytes())
	err = dst.ImportJSON(importBuf, opts)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	dstSettingsCount, _ := dst.GetRowCount("settings")
	dstConfigsCount, _ := dst.GetRowCount("configs")

	if dstSettingsCount != srcSettingsCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcSettingsCount, dstSettingsCount)
	}

	if dstConfigsCount != srcConfigsCount {
		t.Errorf("Configs count mismatch: expected %d, got %d", srcConfigsCount, dstConfigsCount)
	}
}

func TestExportProgressCallback(t *testing.T) {
	client, cleanup := createTestDB(t)
	defer cleanup()

	tables := []string{}
	counts := map[string]int{}

	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferConfig,
		OnProgress: func(table string, count int) {
			tables = append(tables, table)
			counts[table] = count
		},
	}

	err := client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Progress callback should have been called")
	}

	expectedTables := map[string]bool{
		"settings": true,
		"configs":  true,
		"caches":   true,
	}

	for _, table := range tables {
		if !expectedTables[table] {
			t.Errorf("Unexpected table in progress callback: %s", table)
		}
	}
}

func TestEscapeSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"O'Brien", "O''Brien"},
		{"it's a test", "it''s a test"},
	}

	for _, tt := range tests {
		result := escapeSQL(tt.input)
		if result != tt.expected {
			t.Errorf("escapeSQL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatValueForSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, "NULL"},
		{"string", "hello", "'hello'"},
		{"float64", 3.14, "3.14"},
		{"int", 42, "42"},
		{"bool true", true, "1"},
		{"bool false", false, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValueForSQL(tt.input, "")
			if result != tt.expected {
				t.Errorf("formatValueForSQL(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
