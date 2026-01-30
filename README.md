# evccdb

A professional Go package for selective backup, restore, and transfer of [evcc](https://evcc.io/) SQLite database data. Supports transferring data between database instances and selective export/import of configuration or metrics.

## Features

- **Selective Transfer**: Transfer configuration tables or metrics independently
- **JSON Export/Import**: Human-readable JSON format for backups and data exchange
- **Rename**: Rename loadpoints/vehicles across all tables (sessions, settings, configs)
- **Delete Sessions**: Remove session data for specific loadpoints or vehicles
- **Schema-Aware**: Dynamically detects and handles schema differences between databases
- **Dry-Run Mode**: Preview operations without making changes
- **Transaction Safety**: Atomic operations with automatic rollback on error
- **CLI Tool**: Command-line interface for common operations
- **Progress Tracking**: Optional callbacks to monitor transfer progress

## Installation

### Library

```bash
go get github.com/iseebe/evccdb
```

### CLI Tool

```bash
go install github.com/iseebe/evccdb/cmd/evccdb@latest
```

## Quick Start

### Export Configuration

```bash
evccdb export --source evcc.db --output backup.json --mode config
```

### Import Configuration

```bash
evccdb import --source backup.json --target new-evcc.db --mode config
```

### Transfer Between Databases

```bash
evccdb transfer --from old.db --to new.db --mode config
```

### Preview Transfer (Dry Run)

```bash
evccdb transfer --from old.db --to new.db --mode config --dry-run
```

## Transfer Modes

### Config Mode (`--mode config`)
Transfers configuration tables: `settings`, `configs`, `caches`

**Use case**: Migrate user configuration to a new installation

Example:
```bash
evccdb transfer --from old.db --to new.db --mode config
```

### Metrics Mode (`--mode metrics`)
Transfers metrics tables: `meters`, `sessions`, `grid_sessions`

**Use case**: Migrate historical data to a new installation

Example:
```bash
evccdb transfer --from old.db --to new.db --mode metrics
```

### All Mode (`--mode all`)
Transfers all tables

**Use case**: Complete database clone/backup

Example:
```bash
evccdb transfer --from old.db --to new.db --mode all
```

## Custom Tables

Transfer specific tables by name:

```bash
evccdb transfer --from old.db --to new.db --tables settings,configs
```

## Verbose Output

Show transfer progress:

```bash
evccdb transfer --from old.db --to new.db --mode config --verbose
```

## Library Usage

### Basic Example

```go
package main

import (
    "context"
    "github.com/iseebe/evccdb"
)

func main() {
    // Open source and destination databases
    src, _ := evccdb.Open("old.db")
    defer src.Close()

    dst, _ := evccdb.Open("new.db")
    defer dst.Close()

    // Transfer configuration tables
    ctx := context.Background()
    opts := evccdb.TransferOptions{
        Mode: evccdb.TransferConfig,
    }

    evccdb.Transfer(ctx, src, dst, opts)
}
```

### Export to JSON

```go
import (
    "os"
    "github.com/iseebe/evccdb"
)

client, _ := evccdb.Open("evcc.db")
defer client.Close()

f, _ := os.Create("backup.json")
defer f.Close()

opts := evccdb.TransferOptions{
    Mode: evccdb.TransferConfig,
}

client.ExportJSON(f, opts)
```

### Import from JSON

```go
import (
    "os"
    "github.com/iseebe/evccdb"
)

client, _ := evccdb.Open("evcc.db")
defer client.Close()

f, _ := os.Open("backup.json")
defer f.Close()

opts := evccdb.TransferOptions{
    Mode: evccdb.TransferConfig,
}

client.ImportJSON(f, opts)
```

### With Progress Tracking

```go
opts := evccdb.TransferOptions{
    Mode: evccdb.TransferConfig,
    OnProgress: func(table string, count int) {
        fmt.Printf("Transferred %s: %d rows\n", table, count)
    },
}

evccdb.Transfer(ctx, src, dst, opts)
```

### Rename Loadpoint/Vehicle

```go
client, _ := evccdb.Open("evcc.db")
defer client.Close()

ctx := context.Background()

// Rename loadpoint across sessions, settings, and configs
result, _ := client.RenameLoadpoint(ctx, "Garage", "Carport")
fmt.Printf("Renamed: %d sessions, %d settings, %d configs\n",
    result.Sessions, result.Settings, result.Configs)

// Rename vehicle
client.RenameVehicle(ctx, "e-Golf", "ID.4")

// Dry run (preview without changes)
result, _ = client.RenameLoadpointDryRun(ctx, "OldName", "NewName")
```

### Delete Sessions

```go
client, _ := evccdb.Open("evcc.db")
defer client.Close()

ctx := context.Background()

// Count sessions before deleting
count, _ := client.CountLoadpointSessions(ctx, "OldLoadpoint")
fmt.Printf("Found %d sessions\n", count)

// Delete sessions for a loadpoint
deleted, _ := client.DeleteLoadpointSessions(ctx, "OldLoadpoint")
fmt.Printf("Deleted %d sessions\n", deleted)

// Delete sessions for a vehicle
client.DeleteVehicleSessions(ctx, "OldVehicle")
```

### Transfer with Renames

```go
opts := evccdb.TransferOptions{
    Mode: evccdb.TransferAll,
    LoadpointRenames: []evccdb.RenameMapping{
        {OldName: "Garage", NewName: "Carport"},
    },
    VehicleRenames: []evccdb.RenameMapping{
        {OldName: "e-Golf", NewName: "ID.4"},
    },
}

evccdb.Transfer(ctx, src, dst, opts)
```

## Schema Compatibility

The library handles schema differences gracefully:

- **New columns in destination**: Retain their DEFAULT values
- **Missing columns in destination**: Data is skipped with a warning
- **Extra columns in source**: Ignored, only common columns transferred
- **Type mismatches**: Data is transferred as-is (SQLite is flexible)

This approach allows transferring between different evcc versions without requiring exact schema matches.

## Database Schema

### Configuration Tables

**settings** - Key-value pairs (e.g., `savings.started`, `savings.gridCharged`)
```
key    | value
-------|-------
string | string
```

**configs** - Device and service configurations
```
id       | class | type   | value  | title  | icon   | product
---------|-------|--------|--------|--------|--------|--------
integer  | int   | string | string | string | string | string
```

**caches** - Cache entries
```
key    | value
-------|-------
string | string
```

### Metrics Tables

**meters** - Time-series meter readings
```
meter   | ts       | val
--------|----------|--------
integer | datetime | float64
```

**sessions** - Charging sessions
```
id        | created  | finished | loadpoint | identifier | vehicle | ...
----------|----------|----------|-----------|------------|---------|----
integer   | datetime | datetime | string    | string     | string  | ...
```

**grid_sessions** - Grid power sessions
```
id      | created  | finished | type   | grid_power | limit_power
--------|----------|----------|--------|------------|------------
integer | datetime | datetime | string | float64    | float64
```

## Command Reference

### export

Export database tables to JSON.

```
Flags:
  --source string    Source database file (required)
  --output string    Output JSON file (required)
  --mode string      Transfer mode: config, metrics, all (default "config")
  --tables string    Comma-separated table names (overrides mode)
  --verbose          Show progress
```

Examples:
```bash
# Export configuration (settings, configs, caches)
evccdb export --source evcc.db --output config-backup.json --mode config --verbose

# Export session/metrics data (meters, sessions, grid_sessions)
evccdb export --source evcc.db --output metrics-backup.json --mode metrics --verbose

# Export everything
evccdb export --source evcc.db --output full-backup.json --mode all

# Export specific tables only
evccdb export --source evcc.db --output sessions-only.json --tables sessions
evccdb export --source evcc.db --output sessions-meters.json --tables sessions,meters
```

### import

Import JSON data into database.

```
Flags:
  --source string    Source JSON file (required)
  --target string    Target database file (required)
  --mode string      Transfer mode: config, metrics, all (default "config")
  --tables string    Comma-separated table names (overrides mode)
  --verbose          Show progress
```

Examples:
```bash
# Import configuration
evccdb import --source config-backup.json --target evcc.db --mode config

# Import session data
evccdb import --source metrics-backup.json --target evcc.db --mode metrics

# Import specific tables from a full backup
evccdb import --source full-backup.json --target evcc.db --tables sessions
```

### transfer

Transfer data between databases.

```
Flags:
  --from string              Source database file (required)
  --to string                Target database file (required)
  --mode string              Transfer mode: config, metrics, all (default "config")
  --tables string            Comma-separated table names (overrides mode)
  --rename-loadpoint string  Rename loadpoints: OldName:NewName,Old2:New2
  --rename-vehicle string    Rename vehicles: OldName:NewName,Old2:New2
  --dry-run                  Show what would be transferred without doing it
  --verbose                  Show progress
```

Examples:
```bash
# Basic transfer
evccdb transfer --from old.db --to new.db --mode config --dry-run

# Transfer with renames
evccdb transfer --from old.db --to new.db --mode all \
    --rename-loadpoint "Garage:Carport" \
    --rename-vehicle "e-Golf:ID.4"
```

### rename

Rename loadpoints or vehicles across all tables (sessions, settings, configs).

```
Flags:
  --db string         Database file (required)
  --loadpoint string  Rename loadpoints: OldName:NewName,Old2:New2
  --vehicle string    Rename vehicles: OldName:NewName,Old2:New2
  --dry-run           Show what would be renamed without doing it
  --verbose           Show detailed output
```

Examples:
```bash
# Preview rename
evccdb rename --db evcc.db --loadpoint "Garage:Carport" --dry-run

# Rename loadpoint
evccdb rename --db evcc.db --loadpoint "Garage:Carport" --verbose

# Rename vehicle
evccdb rename --db evcc.db --vehicle "e-Golf:ID.4"

# Multiple renames
evccdb rename --db evcc.db --loadpoint "Garage:Carport,eBikes:E-Bikes"
```

### delete

Delete session data for specific loadpoints or vehicles.

**Warning**: Make sure evcc is stopped before running this command.

```
Flags:
  --db string         Database file (required)
  --loadpoint string  Delete sessions for loadpoints: Name1,Name2
  --vehicle string    Delete sessions for vehicles: Name1,Name2
  --dry-run           Show what would be deleted without doing it
  -y, --yes           Skip confirmation prompt
  --verbose           Show detailed output
```

Examples:
```bash
# Preview deletion
evccdb delete --db evcc.db --loadpoint "OldLoadpoint" --dry-run

# Delete sessions (with confirmation prompt)
evccdb delete --db evcc.db --loadpoint "OldLoadpoint"

# Delete without confirmation (for scripts)
evccdb delete --db evcc.db --loadpoint "OldLoadpoint" -y

# Delete multiple
evccdb delete --db evcc.db --loadpoint "LP1,LP2" --vehicle "Vehicle1"
```

## Testing

Run tests:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -v -race -coverprofile=coverage.out ./...
```

Run only unit tests (skip integration tests):

```bash
go test -v -short ./...
```

## License

MIT - See LICENSE file for details

## Contributing

This project is part of the evcc ecosystem. For issues and feature requests, please use the project issue tracker.

## Disclaimer

Use at own risk. Actions taken cannot be undone. Create backups before doing modifications and ensure no parallel access on database file.
