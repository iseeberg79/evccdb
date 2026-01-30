package evccdb

import (
	"os"
	"testing"
)

// createTestDB creates a temporary test database with sample data
func createTestDB(t *testing.T) (*Client, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "evccdb-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()

	client, err := Open(tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE configs (id INTEGER PRIMARY KEY, class INTEGER, type TEXT, value TEXT, title TEXT, icon TEXT, product TEXT);
		CREATE TABLE caches (key TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE meters (meter INTEGER, ts DATETIME, val REAL);
		CREATE UNIQUE INDEX meter_ts ON meters(meter, ts);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY,
			created DATETIME,
			finished DATETIME,
			loadpoint TEXT,
			identifier TEXT,
			vehicle TEXT,
			odometer REAL,
			meter_start_kwh REAL,
			meter_end_kwh REAL,
			charged_kwh REAL,
			solar_percentage REAL,
			price REAL,
			price_per_kwh REAL,
			co2_per_kwh REAL,
			charge_duration INTEGER
		);
		CREATE TABLE grid_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created DATETIME,
			finished DATETIME,
			type TEXT,
			grid_power REAL,
			limit_power REAL
		);
	`
	if _, err := client.db.Exec(schema); err != nil {
		_ = client.Close()
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert sample data
	sampleData := `
		INSERT INTO settings (key, value) VALUES
			('lp1.title', 'Garage'),
			('lp1.mode', 'pv'),
			('lp2.title', 'eBikes'),
			('vehicle.e-Golf.minSoc', '25'),
			('vehicle.e-Golf.limitSoc', '90'),
			('vehicle.e-Golf.planSoc', '0');

		INSERT INTO configs (id, class, type, value) VALUES
			(1, 5, 'template', '{"title":"Garage","charger":"db:1"}'),
			(2, 3, 'template', '{"title":"e-Golf","type":"vw"}');

		INSERT INTO sessions (id, created, loadpoint, vehicle) VALUES
			(1, '2023-04-01 10:00:00', 'Garage', 'e-Golf'),
			(2, '2023-04-02 10:00:00', 'Garage', 'e-Golf'),
			(3, '2023-04-03 10:00:00', 'Garage', NULL),
			(4, '2023-04-04 10:00:00', 'eBikes', 'e-Bike'),
			(5, '2023-04-05 10:00:00', 'eBikes', NULL);
	`
	if _, err := client.db.Exec(sampleData); err != nil {
		_ = client.Close()
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("Failed to insert sample data: %v", err)
	}

	cleanup := func() {
		_ = client.Close()
		_ = os.Remove(tmpFile.Name())
	}

	return client, cleanup
}
