# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go library and CLI tool for selective backup, restore, transfer, and maintenance of [evcc](https://evcc.io/) SQLite databases. Supports renaming loadpoints/vehicles and deleting session data.

## Build & Test Commands

```bash
# Run all tests
go test -v ./...

# Run tests with race detection and coverage
go test -v -race -coverprofile=coverage.out ./...

# Build library
go build -v ./...

# Build CLI tool
go build -o tmp/evccdb ./cmd/evccdb/

# Format code
go fmt ./...

# Run linter
golangci-lint run --timeout 5m
```

## Architecture

### Core Components

**Client (`client.go`)**: Database connection wrapper. Opens SQLite database, provides table introspection methods.

**Transfer (`transfer.go`)**: Copies data between databases with schema compatibility handling. Supports dry-run mode.

**Rename (`rename.go`)**: Renames loadpoints/vehicles across all tables:
- `sessions` table: UPDATE loadpoint/vehicle column
- `settings` table: UPDATE values (loadpoint titles) or rename keys (vehicle.X.*)
- `configs` table: UPDATE JSON title field

**Export/Import (`export.go`, `import.go`)**: JSON serialization for backup/restore.

**Types (`types.go`)**: Shared types including `TransferOptions`, `RenameMapping`, `RenameResult`.

### Database Tables

**Config tables**: `settings`, `configs`, `caches`
**Metrics tables**: `meters`, `sessions`, `grid_sessions`

### Key Patterns

- Loadpoint names stored in: `sessions.loadpoint`, `settings` (lp%.title values), `configs` class 5 JSON
- Vehicle names stored in: `sessions.vehicle`, `settings` (vehicle.X.* keys), `configs` class 3 JSON

## CLI Commands

```bash
# Export/Import
evccdb export --source evcc.db --output backup.json --mode config
evccdb import --source backup.json --target evcc.db

# Transfer between databases
evccdb transfer --from old.db --to new.db --mode all

# Rename loadpoint/vehicle
evccdb rename --db evcc.db --loadpoint "Old:New" --dry-run
evccdb rename --db evcc.db --vehicle "e-Golf:ID.4"

# Delete sessions
evccdb delete --db evcc.db --loadpoint "Name" --dry-run
evccdb delete --db evcc.db --loadpoint "Name" -y
```

## Package Structure

- Root: Library package `evccdb`
- `cmd/evccdb/`: CLI tool
- `testdata/`: Test database
