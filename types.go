package evccdb

import "io"

// TransferMode specifies which tables to transfer
type TransferMode int

const (
	TransferConfig TransferMode = iota
	TransferMetrics
	TransferAll
)

// RenameMapping defines a name transformation
type RenameMapping struct {
	OldName string
	NewName string
}

// TransferOptions configures transfer behavior
type TransferOptions struct {
	Mode             TransferMode
	Tables           []string
	DryRun           bool
	OnProgress       func(table string, count int)
	LoadpointRenames []RenameMapping
	VehicleRenames   []RenameMapping
}

// Setting represents a key-value configuration pair
type Setting struct {
	Key   string
	Value string
}

// Config represents a device or service configuration
type Config struct {
	ID      int
	Class   int
	Type    string
	Value   string
	Title   string
	Icon    string
	Product string
}

// Meter represents a meter reading at a specific time
type Meter struct {
	Meter int
	Ts    string
	Val   float64
}

// Session represents a charging session
type Session struct {
	ID              int
	Created         string
	Finished        *string
	Loadpoint       string
	Identifier      *string
	Vehicle         *string
	OdometerStart   *float64
	MeterStartKwh   *float64
	MeterEndKwh     *float64
	ChargedKwh      *float64
	SolarPercentage *float64
	Price           *float64
	PricePerKwh     *float64
	Co2PerKwh       *float64
	ChargeDuration  *int
}

// GridSession represents a grid power session
type GridSession struct {
	ID         int
	Created    string
	Finished   *string
	Type       string
	GridPower  *float64
	LimitPower *float64
}

// ExportFormat is the JSON structure for export/import
type ExportFormat struct {
	Version    string         `json:"version"`
	ExportedAt string         `json:"exported_at"`
	Tables     map[string]any `json:"tables"`
}

// Exporter defines the export interface
type Exporter interface {
	ExportJSON(w io.Writer, opts TransferOptions) error
}

// Importer defines the import interface
type Importer interface {
	ImportJSON(r io.Reader, opts TransferOptions) error
}
