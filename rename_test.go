package evccdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRenameLoadpointInSessions(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Verify initial state
	var count int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	initialCount := count

	// Perform rename
	result, err := client.RenameLoadpoint(ctx, "Garage", "Carport")
	if err != nil {
		t.Fatalf("RenameLoadpoint failed: %v", err)
	}

	if result.Sessions != initialCount {
		t.Errorf("Expected %d sessions renamed, got %d", initialCount, result.Sessions)
	}

	// Verify rename happened
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 sessions with old name, got %d", count)
	}

	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Carport'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != initialCount {
		t.Errorf("Expected %d sessions with new name, got %d", initialCount, count)
	}
}

func TestRenameLoadpointInSettings(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Check if the settings value exists
	var value string
	err := client.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'lp1.title'").Scan(&value)
	if err != nil {
		t.Skipf("No lp1.title setting found, skipping test")
	}

	if value != "Garage" {
		t.Skipf("lp1.title is not 'Garage' (is %q), skipping test", value)
	}

	// Perform rename
	result, err := client.RenameLoadpoint(ctx, "Garage", "Carport")
	if err != nil {
		t.Fatalf("RenameLoadpoint failed: %v", err)
	}

	if result.Settings == 0 {
		t.Error("Expected at least 1 setting renamed")
	}

	// Verify rename happened
	err = client.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'lp1.title'").Scan(&value)
	if err != nil {
		t.Fatalf("Failed to get setting: %v", err)
	}
	if value != "Carport" {
		t.Errorf("Expected 'Carport', got %q", value)
	}
}

func TestRenameVehicleInSessions(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Verify initial state
	var count int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = 'e-Golf'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	initialCount := count

	if initialCount == 0 {
		t.Skip("No sessions with vehicle 'e-Golf' found")
	}

	// Perform rename
	result, err := client.RenameVehicle(ctx, "e-Golf", "ID.4")
	if err != nil {
		t.Fatalf("RenameVehicle failed: %v", err)
	}

	if result.Sessions != initialCount {
		t.Errorf("Expected %d sessions renamed, got %d", initialCount, result.Sessions)
	}

	// Verify rename happened
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = 'e-Golf'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 sessions with old name, got %d", count)
	}

	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = 'ID.4'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != initialCount {
		t.Errorf("Expected %d sessions with new name, got %d", initialCount, count)
	}
}

func TestRenameVehicleSettingsKeys(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count settings with old prefix
	var count int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'vehicle.e-Golf.%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count settings: %v", err)
	}
	initialCount := count

	if initialCount == 0 {
		t.Skip("No settings with 'vehicle.e-Golf.' prefix found")
	}

	// Perform rename
	result, err := client.RenameVehicle(ctx, "e-Golf", "ID.4")
	if err != nil {
		t.Fatalf("RenameVehicle failed: %v", err)
	}

	if result.Settings != initialCount {
		t.Errorf("Expected %d settings renamed, got %d", initialCount, result.Settings)
	}

	// Verify old keys are gone
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'vehicle.e-Golf.%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count settings: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 settings with old prefix, got %d", count)
	}

	// Verify new keys exist
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'vehicle.ID.4.%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count settings: %v", err)
	}
	if count != initialCount {
		t.Errorf("Expected %d settings with new prefix, got %d", initialCount, count)
	}
}

func TestRenameInConfigsJSON(t *testing.T) {
	client, cleanup := setupTestDBWithConfigs(t)
	defer cleanup()

	ctx := context.Background()

	// Perform rename
	result, err := client.RenameLoadpoint(ctx, "TestLoadpoint", "RenamedLoadpoint")
	if err != nil {
		t.Fatalf("RenameLoadpoint failed: %v", err)
	}

	if result.Configs != 1 {
		t.Errorf("Expected 1 config renamed, got %d", result.Configs)
	}

	// Verify the config was updated
	var value string
	err = client.db.QueryRowContext(ctx, "SELECT value FROM configs WHERE class = 5").Scan(&value)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if data["title"] != "RenamedLoadpoint" {
		t.Errorf("Expected title 'RenamedLoadpoint', got %v", data["title"])
	}
}

func TestRenameLoadpointDryRun(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get initial state
	var initialSessionCount int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&initialSessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	// Perform dry run
	result, err := client.RenameLoadpointDryRun(ctx, "Garage", "Carport")
	if err != nil {
		t.Fatalf("RenameLoadpointDryRun failed: %v", err)
	}

	if result.Sessions != initialSessionCount {
		t.Errorf("Expected dry run to report %d sessions, got %d", initialSessionCount, result.Sessions)
	}

	// Verify no changes were made
	var count int
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != initialSessionCount {
		t.Errorf("Dry run modified data: expected %d, got %d", initialSessionCount, count)
	}
}

func TestRenameVehicleDryRun(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get initial state
	var initialSessionCount int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = 'e-Golf'").Scan(&initialSessionCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	var initialSettingsCount int
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'vehicle.e-Golf.%'").Scan(&initialSettingsCount)
	if err != nil {
		t.Fatalf("Failed to count settings: %v", err)
	}

	// Perform dry run
	result, err := client.RenameVehicleDryRun(ctx, "e-Golf", "ID.4")
	if err != nil {
		t.Fatalf("RenameVehicleDryRun failed: %v", err)
	}

	if result.Sessions != initialSessionCount {
		t.Errorf("Expected dry run to report %d sessions, got %d", initialSessionCount, result.Sessions)
	}

	if result.Settings != initialSettingsCount {
		t.Errorf("Expected dry run to report %d settings, got %d", initialSettingsCount, result.Settings)
	}

	// Verify no changes were made
	var count int
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE vehicle = 'e-Golf'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != initialSessionCount {
		t.Errorf("Dry run modified sessions: expected %d, got %d", initialSessionCount, count)
	}

	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key LIKE 'vehicle.e-Golf.%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count settings: %v", err)
	}
	if count != initialSettingsCount {
		t.Errorf("Dry run modified settings: expected %d, got %d", initialSettingsCount, count)
	}
}

func TestTransferWithRenames(t *testing.T) {
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

	// Get initial counts from source
	var srcGarageCount int
	err = src.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&srcGarageCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	// Perform transfer with rename
	ctx := context.Background()
	opts := TransferOptions{
		Mode: TransferAll,
		LoadpointRenames: []RenameMapping{
			{OldName: "Garage", NewName: "Carport"},
		},
	}

	err = Transfer(ctx, src, dst, opts)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	// Verify rename was applied in destination
	var dstCarportCount int
	err = dst.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Carport'").Scan(&dstCarportCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if dstCarportCount != srcGarageCount {
		t.Errorf("Expected %d sessions with 'Carport', got %d", srcGarageCount, dstCarportCount)
	}

	// Verify no 'Garage' sessions in destination
	var dstGarageCount int
	err = dst.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&dstGarageCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if dstGarageCount != 0 {
		t.Errorf("Expected 0 sessions with 'Garage', got %d", dstGarageCount)
	}
}

func TestRenameNonExistent(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Rename a non-existent loadpoint
	result, err := client.RenameLoadpoint(ctx, "NonExistent", "NewName")
	if err != nil {
		t.Fatalf("RenameLoadpoint should not fail for non-existent: %v", err)
	}

	if result.Sessions != 0 || result.Settings != 0 || result.Configs != 0 {
		t.Errorf("Expected all zeros for non-existent rename, got sessions=%d, settings=%d, configs=%d",
			result.Sessions, result.Settings, result.Configs)
	}
}

func TestMultipleRenames(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count initial sessions for both loadpoints
	var garageCount, eBikesCount int
	err := client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&garageCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'eBikes'").Scan(&eBikesCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	// Perform multiple renames
	_, err = client.RenameLoadpoint(ctx, "Garage", "Carport")
	if err != nil {
		t.Fatalf("First rename failed: %v", err)
	}

	_, err = client.RenameLoadpoint(ctx, "eBikes", "E-Bikes")
	if err != nil {
		t.Fatalf("Second rename failed: %v", err)
	}

	// Verify both renames happened
	var carportCount, eBikesNewCount int
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Carport'").Scan(&carportCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'E-Bikes'").Scan(&eBikesNewCount)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if carportCount != garageCount {
		t.Errorf("Expected %d sessions with 'Carport', got %d", garageCount, carportCount)
	}
	if eBikesNewCount != eBikesCount {
		t.Errorf("Expected %d sessions with 'E-Bikes', got %d", eBikesCount, eBikesNewCount)
	}
}

// Helper functions

func setupTestDB(t *testing.T) (*Client, func()) {
	// Copy test database to temp file
	srcDB, err := os.ReadFile("testdata/test.db")
	if err != nil {
		t.Fatalf("Failed to read test database: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "rename-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write(srcDB); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	client, err := Open(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open temp database: %v", err)
	}

	cleanup := func() {
		client.Close()
		os.Remove(tmpFile.Name())
	}

	return client, cleanup
}

func setupTestDBWithConfigs(t *testing.T) (*Client, func()) {
	client, cleanup := setupTestDB(t)

	// Insert a test config with JSON value
	configJSON := `{"title":"TestLoadpoint","charger":"db:1"}`
	_, err := client.db.Exec("INSERT INTO configs (class, type, value) VALUES (5, 'template', ?)", configJSON)
	if err != nil {
		cleanup()
		t.Fatalf("Failed to insert test config: %v", err)
	}

	return client, cleanup
}

func TestRenameSettingsKeysPreservesValues(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get the value of a setting before rename
	var minSocBefore string
	err := client.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'vehicle.e-Golf.minSoc'").Scan(&minSocBefore)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Skip("No vehicle.e-Golf.minSoc setting found")
		}
		t.Fatalf("Failed to get setting: %v", err)
	}

	// Perform rename
	_, err = client.RenameVehicle(ctx, "e-Golf", "ID.4")
	if err != nil {
		t.Fatalf("RenameVehicle failed: %v", err)
	}

	// Verify the value is preserved with new key
	var minSocAfter string
	err = client.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = 'vehicle.ID.4.minSoc'").Scan(&minSocAfter)
	if err != nil {
		t.Fatalf("Failed to get renamed setting: %v", err)
	}

	if minSocBefore != minSocAfter {
		t.Errorf("Value changed during rename: before=%q, after=%q", minSocBefore, minSocAfter)
	}
}

func TestRenameYAMLStyleConfig(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a YAML-style config
	yamlConfig := "title: YAMLVehicle\ntype: template\nother: value"
	_, err := client.db.Exec("INSERT INTO configs (class, type, value) VALUES (3, 'template', ?)", yamlConfig)
	if err != nil {
		t.Fatalf("Failed to insert YAML config: %v", err)
	}

	ctx := context.Background()

	// Perform rename
	result, err := client.RenameVehicle(ctx, "YAMLVehicle", "RenamedYAML")
	if err != nil {
		t.Fatalf("RenameVehicle failed: %v", err)
	}

	if result.Configs != 1 {
		t.Errorf("Expected 1 config renamed, got %d", result.Configs)
	}

	// Verify the config was updated
	var value string
	rows, err := client.db.QueryContext(ctx, "SELECT value FROM configs WHERE class = 3")
	if err != nil {
		t.Fatalf("Failed to query configs: %v", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("Failed to scan value: %v", err)
		}
		if strings.Contains(value, "title: RenamedYAML") {
			found = true
			break
		}
	}

	if !found {
		t.Error("YAML config was not renamed")
	}
}

func TestDeleteLoadpointSessions(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get initial count
	initialCount, err := client.CountLoadpointSessions(ctx, "Garage")
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if initialCount == 0 {
		t.Skip("No sessions for 'Garage' loadpoint found")
	}

	// Delete sessions
	deleted, err := client.DeleteLoadpointSessions(ctx, "Garage")
	if err != nil {
		t.Fatalf("DeleteLoadpointSessions failed: %v", err)
	}

	if deleted != initialCount {
		t.Errorf("Expected to delete %d sessions, deleted %d", initialCount, deleted)
	}

	// Verify deletion
	remaining, err := client.CountLoadpointSessions(ctx, "Garage")
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if remaining != 0 {
		t.Errorf("Expected 0 sessions remaining, got %d", remaining)
	}
}

func TestDeleteVehicleSessions(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Get initial count
	initialCount, err := client.CountVehicleSessions(ctx, "e-Golf")
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if initialCount == 0 {
		t.Skip("No sessions for 'e-Golf' vehicle found")
	}

	// Delete sessions
	deleted, err := client.DeleteVehicleSessions(ctx, "e-Golf")
	if err != nil {
		t.Fatalf("DeleteVehicleSessions failed: %v", err)
	}

	if deleted != initialCount {
		t.Errorf("Expected to delete %d sessions, deleted %d", initialCount, deleted)
	}

	// Verify deletion
	remaining, err := client.CountVehicleSessions(ctx, "e-Golf")
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}

	if remaining != 0 {
		t.Errorf("Expected 0 sessions remaining, got %d", remaining)
	}
}

func TestDeleteNonExistentLoadpoint(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	deleted, err := client.DeleteLoadpointSessions(ctx, "NonExistentLoadpoint")
	if err != nil {
		t.Fatalf("DeleteLoadpointSessions should not fail for non-existent: %v", err)
	}

	if deleted != 0 {
		t.Errorf("Expected 0 sessions deleted, got %d", deleted)
	}
}

func TestCountLoadpointSessions(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	count, err := client.CountLoadpointSessions(ctx, "Garage")
	if err != nil {
		t.Fatalf("CountLoadpointSessions failed: %v", err)
	}

	if count <= 0 {
		t.Skip("No sessions for 'Garage' loadpoint found")
	}

	// Count should match direct query
	var directCount int
	err = client.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE loadpoint = 'Garage'").Scan(&directCount)
	if err != nil {
		t.Fatalf("Direct count failed: %v", err)
	}

	if count != directCount {
		t.Errorf("CountLoadpointSessions=%d does not match direct count=%d", count, directCount)
	}
}
