package evccdb

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestExportJSON(t *testing.T) {
	client, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferConfig,
	}

	err = client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Verify JSON structure
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

	if len(export.Tables) == 0 {
		t.Error("No tables in export")
	}

	// Verify config tables are present
	configTables := []string{"settings", "configs", "caches"}
	for _, table := range configTables {
		if _, exists := export.Tables[table]; !exists {
			t.Errorf("Expected table %s in export", table)
		}
	}
}

func TestExportJSONMetrics(t *testing.T) {
	client, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferMetrics,
	}

	err = client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Verify JSON structure
	var export ExportFormat
	err = json.Unmarshal(buf.Bytes(), &export)
	if err != nil {
		t.Fatalf("Failed to unmarshal exported JSON: %v", err)
	}

	// Verify metrics tables are present
	metricsTables := []string{"meters", "sessions", "grid_sessions"}
	for _, table := range metricsTables {
		if _, exists := export.Tables[table]; !exists {
			t.Errorf("Expected table %s in export", table)
		}
	}
}

func TestExportJSONAll(t *testing.T) {
	client, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferAll,
	}

	err = client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Verify JSON structure
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
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Export data
	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferConfig,
	}

	err = src.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema
	initializeDB(t, tmpFile.Name(), src)

	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Get count from source
	srcCount, _ := src.GetRowCount("settings")

	// Import data
	importBuf := bytes.NewReader(buf.Bytes())
	opts = TransferOptions{
		Mode: TransferConfig,
	}

	err = dst.ImportJSON(importBuf, opts)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	// Verify counts
	dstCount, _ := dst.GetRowCount("settings")
	if dstCount != srcCount {
		t.Errorf("Settings count mismatch: expected %d, got %d", srcCount, dstCount)
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	src, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Get initial counts
	srcSettingsCount, _ := src.GetRowCount("settings")
	srcConfigsCount, _ := src.GetRowCount("configs")

	// Export
	var buf bytes.Buffer
	opts := TransferOptions{
		Mode: TransferConfig,
	}

	err = src.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Create destination database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Initialize destination with schema
	initializeDB(t, tmpFile.Name(), src)

	dst, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Import
	var importBuf *bytes.Reader
	importBuf = bytes.NewReader(buf.Bytes())
	opts = TransferOptions{
		Mode: TransferConfig,
	}

	err = dst.ImportJSON(importBuf, opts)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	// Verify counts
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
	client, err := Open("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer client.Close()

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

	err = client.ExportJSON(&buf, opts)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	if len(tables) == 0 {
		t.Fatal("Progress callback should have been called")
	}

	// Verify all config tables were processed
	expectedTables := map[string]bool{
		"settings": true,
		"configs":  true,
		"caches":   true,
	}

	for _, table := range tables {
		if !expectedTables[table] {
			t.Errorf("Unexpected table in progress callback: %s", table)
		}
		if counts[table] < 0 {
			t.Errorf("Table %s should have non-negative count", table)
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
		{"don't stop", "don''t stop"},
		{"''already''", "''''already''''"},
		{"no quotes here", "no quotes here"},
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
		{"string with quote", "O'Brien", "'O''Brien'"},
		{"float64", 3.14, "3.14"},
		{"int", 42, "42"},
		{"bool true", true, "1"},
		{"bool false", false, "0"},
		{"unknown type", struct{}{}, "NULL"},
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
