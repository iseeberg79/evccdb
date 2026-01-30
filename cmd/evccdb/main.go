package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/iseebe/evccdb"
	"github.com/spf13/cobra"
)

var (
	source           string
	target           string
	output           string
	modeStr          string
	tables           string
	dryRun           bool
	verbose          bool
	transferSrc      string
	transferDst      string
	renameLoadpoints string
	renameVehicles   string
	renameDB         string
	deleteDB         string
	deleteLoadpoints string
	deleteVehicles   string
	assumeYes        bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "evccdb",
		Short: "Tool for evcc database backup and transfer",
		Long:  "evccdb provides selective backup, restore, and transfer of evcc SQLite database data",
	}

	// Export command
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export database tables to JSON",
		RunE:  runExport,
	}
	exportCmd.Flags().StringVar(&source, "source", "", "Source database file (required)")
	exportCmd.Flags().StringVar(&output, "output", "", "Output JSON file (required)")
	exportCmd.Flags().StringVar(&modeStr, "mode", "config", "Transfer mode: config, metrics, all")
	exportCmd.Flags().StringVar(&tables, "tables", "", "Comma-separated table names (overrides mode)")
	exportCmd.Flags().BoolVar(&verbose, "verbose", false, "Show progress")
	_ = exportCmd.MarkFlagRequired("source")
	_ = exportCmd.MarkFlagRequired("output")

	// Import command
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import JSON data into database",
		RunE:  runImport,
	}
	importCmd.Flags().StringVar(&source, "source", "", "Source JSON file (required)")
	importCmd.Flags().StringVar(&target, "target", "", "Target database file (required)")
	importCmd.Flags().StringVar(&modeStr, "mode", "config", "Transfer mode: config, metrics, all")
	importCmd.Flags().StringVar(&tables, "tables", "", "Comma-separated table names (overrides mode)")
	importCmd.Flags().BoolVar(&verbose, "verbose", false, "Show progress")
	_ = importCmd.MarkFlagRequired("source")
	_ = importCmd.MarkFlagRequired("target")

	// Transfer command
	transferCmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer data between databases",
		RunE:  runTransfer,
	}
	transferCmd.Flags().StringVar(&transferSrc, "from", "", "Source database file (required)")
	transferCmd.Flags().StringVar(&transferDst, "to", "", "Target database file (required)")
	transferCmd.Flags().StringVar(&modeStr, "mode", "config", "Transfer mode: config, metrics, all")
	transferCmd.Flags().StringVar(&tables, "tables", "", "Comma-separated table names (overrides mode)")
	transferCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be transferred without doing it")
	transferCmd.Flags().BoolVar(&verbose, "verbose", false, "Show progress")
	transferCmd.Flags().StringVar(&renameLoadpoints, "rename-loadpoint", "", "Rename loadpoints: OldName:NewName,OldName2:NewName2")
	transferCmd.Flags().StringVar(&renameVehicles, "rename-vehicle", "", "Rename vehicles: OldName:NewName,OldName2:NewName2")
	_ = transferCmd.MarkFlagRequired("from")
	_ = transferCmd.MarkFlagRequired("to")

	// Rename command
	renameCmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename loadpoints or vehicles in database",
		RunE:  runRename,
	}
	renameCmd.Flags().StringVar(&renameDB, "db", "", "Database file (required)")
	renameCmd.Flags().StringVar(&renameLoadpoints, "loadpoint", "", "Rename loadpoints: OldName:NewName,OldName2:NewName2")
	renameCmd.Flags().StringVar(&renameVehicles, "vehicle", "", "Rename vehicles: OldName:NewName,OldName2:NewName2")
	renameCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be renamed without doing it")
	renameCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output")
	_ = renameCmd.MarkFlagRequired("db")

	// Delete command
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete session data for loadpoints or vehicles",
		Long: `Delete session data for specific loadpoints or vehicles.

WARNING: This operation is destructive and cannot be undone.
Make sure evcc is stopped and not accessing the database before running this command.`,
		RunE: runDelete,
	}
	deleteCmd.Flags().StringVar(&deleteDB, "db", "", "Database file (required)")
	deleteCmd.Flags().StringVar(&deleteLoadpoints, "loadpoint", "", "Delete sessions for loadpoints: Name1,Name2")
	deleteCmd.Flags().StringVar(&deleteVehicles, "vehicle", "", "Delete sessions for vehicles: Name1,Name2")
	deleteCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without doing it")
	deleteCmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output")
	_ = deleteCmd.MarkFlagRequired("db")

	rootCmd.AddCommand(exportCmd, importCmd, transferCmd, renameCmd, deleteCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExport(cmd *cobra.Command, args []string) error {
	client, err := evccdb.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = client.Close() }()

	mode := parseMode(modeStr)
	opts := evccdb.TransferOptions{
		Mode: mode,
	}

	if tables != "" {
		opts.Tables = strings.Split(tables, ",")
		for i := range opts.Tables {
			opts.Tables[i] = strings.TrimSpace(opts.Tables[i])
		}
	}

	if verbose {
		opts.OnProgress = func(table string, count int) {
			fmt.Printf("Exported %s: %d rows\n", table, count)
		}
	}

	outputFile, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outputFile.Close() }()

	if err := client.ExportJSON(outputFile, opts); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	fmt.Printf("Successfully exported to %s\n", output)
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	client, err := evccdb.Open(target)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = client.Close() }()

	mode := parseMode(modeStr)
	opts := evccdb.TransferOptions{
		Mode: mode,
	}

	if tables != "" {
		opts.Tables = strings.Split(tables, ",")
		for i := range opts.Tables {
			opts.Tables[i] = strings.TrimSpace(opts.Tables[i])
		}
	}

	if verbose {
		opts.OnProgress = func(table string, count int) {
			fmt.Printf("Imported %s: %d rows\n", table, count)
		}
	}

	if err := client.ImportJSON(sourceFile, opts); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("Successfully imported from %s\n", source)
	return nil
}

func runTransfer(cmd *cobra.Command, args []string) error {
	src, err := evccdb.Open(transferSrc)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := evccdb.Open(transferDst)
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer func() { _ = dst.Close() }()

	mode := parseMode(modeStr)
	opts := evccdb.TransferOptions{
		Mode:   mode,
		DryRun: dryRun,
	}

	if tables != "" {
		opts.Tables = strings.Split(tables, ",")
		for i := range opts.Tables {
			opts.Tables[i] = strings.TrimSpace(opts.Tables[i])
		}
	}

	// Parse loadpoint renames
	if renameLoadpoints != "" {
		renames, err := parseRenames(renameLoadpoints)
		if err != nil {
			return fmt.Errorf("invalid --rename-loadpoint: %w", err)
		}
		opts.LoadpointRenames = renames
	}

	// Parse vehicle renames
	if renameVehicles != "" {
		renames, err := parseRenames(renameVehicles)
		if err != nil {
			return fmt.Errorf("invalid --rename-vehicle: %w", err)
		}
		opts.VehicleRenames = renames
	}

	if verbose {
		opts.OnProgress = func(table string, count int) {
			fmt.Printf("Transferred %s: %d rows\n", table, count)
		}
	}

	ctx := context.Background()
	if err := evccdb.Transfer(ctx, src, dst, opts); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	if dryRun {
		fmt.Println("Dry run completed (no changes made)")
	} else {
		fmt.Println("Transfer completed successfully")
	}
	return nil
}

func runRename(cmd *cobra.Command, args []string) error {
	client, err := evccdb.Open(renameDB)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Parse and apply loadpoint renames
	if renameLoadpoints != "" {
		renames, err := parseRenames(renameLoadpoints)
		if err != nil {
			return fmt.Errorf("invalid --loadpoint: %w", err)
		}

		for _, rename := range renames {
			if dryRun {
				result, err := client.RenameLoadpointDryRun(ctx, rename.OldName, rename.NewName)
				if err != nil {
					return fmt.Errorf("dry run failed for loadpoint %q: %w", rename.OldName, err)
				}
				fmt.Printf("Would rename loadpoint %q -> %q: sessions=%d, settings=%d, configs=%d\n",
					rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
			} else {
				result, err := client.RenameLoadpoint(ctx, rename.OldName, rename.NewName)
				if err != nil {
					return fmt.Errorf("failed to rename loadpoint %q: %w", rename.OldName, err)
				}
				if verbose {
					fmt.Printf("Renamed loadpoint %q -> %q: sessions=%d, settings=%d, configs=%d\n",
						rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
				}
			}
		}
	}

	// Parse and apply vehicle renames
	if renameVehicles != "" {
		renames, err := parseRenames(renameVehicles)
		if err != nil {
			return fmt.Errorf("invalid --vehicle: %w", err)
		}

		for _, rename := range renames {
			if dryRun {
				result, err := client.RenameVehicleDryRun(ctx, rename.OldName, rename.NewName)
				if err != nil {
					return fmt.Errorf("dry run failed for vehicle %q: %w", rename.OldName, err)
				}
				fmt.Printf("Would rename vehicle %q -> %q: sessions=%d, settings=%d, configs=%d\n",
					rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
			} else {
				result, err := client.RenameVehicle(ctx, rename.OldName, rename.NewName)
				if err != nil {
					return fmt.Errorf("failed to rename vehicle %q: %w", rename.OldName, err)
				}
				if verbose {
					fmt.Printf("Renamed vehicle %q -> %q: sessions=%d, settings=%d, configs=%d\n",
						rename.OldName, rename.NewName, result.Sessions, result.Settings, result.Configs)
				}
			}
		}
	}

	if dryRun {
		fmt.Println("Dry run completed (no changes made)")
	} else {
		fmt.Println("Rename completed successfully")
	}
	return nil
}

// parseRenames parses "OldName:NewName,OldName2:NewName2" format
func parseRenames(s string) ([]evccdb.RenameMapping, error) {
	if s == "" {
		return nil, nil
	}

	var renames []evccdb.RenameMapping
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid rename format %q, expected OldName:NewName", pair)
		}

		oldName := strings.TrimSpace(parts[0])
		newName := strings.TrimSpace(parts[1])
		if oldName == "" || newName == "" {
			return nil, fmt.Errorf("invalid rename format %q, names cannot be empty", pair)
		}

		renames = append(renames, evccdb.RenameMapping{
			OldName: oldName,
			NewName: newName,
		})
	}

	return renames, nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	if deleteLoadpoints == "" && deleteVehicles == "" {
		return fmt.Errorf("at least one of --loadpoint or --vehicle must be specified")
	}

	// Confirm that evcc is stopped
	if !dryRun && !assumeYes {
		fmt.Print("WARNING: Make sure evcc is stopped and not accessing the database.\n")
		fmt.Print("Type 'yes' to confirm and proceed: ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("Operation cancelled")
			return nil
		}
	}

	client, err := evccdb.Open(deleteDB)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = client.Close() }()
	ctx := context.Background()

	// Parse and delete loadpoint sessions
	if deleteLoadpoints != "" {
		names := parseNames(deleteLoadpoints)
		for _, name := range names {
			if dryRun {
				count, err := client.CountLoadpointSessions(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to count sessions for loadpoint %q: %w", name, err)
				}
				fmt.Printf("Would delete %d sessions for loadpoint %q\n", count, name)
			} else {
				count, err := client.DeleteLoadpointSessions(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to delete sessions for loadpoint %q: %w", name, err)
				}
				fmt.Printf("Deleted %d sessions for loadpoint %q\n", count, name)
			}
		}
	}

	// Parse and delete vehicle sessions
	if deleteVehicles != "" {
		names := parseNames(deleteVehicles)
		for _, name := range names {
			if dryRun {
				count, err := client.CountVehicleSessions(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to count sessions for vehicle %q: %w", name, err)
				}
				fmt.Printf("Would delete %d sessions for vehicle %q\n", count, name)
			} else {
				count, err := client.DeleteVehicleSessions(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to delete sessions for vehicle %q: %w", name, err)
				}
				fmt.Printf("Deleted %d sessions for vehicle %q\n", count, name)
			}
		}
	}

	if dryRun {
		fmt.Println("Dry run completed (no changes made)")
	} else {
		fmt.Println("Delete completed successfully")
	}
	return nil
}

// parseNames parses comma-separated names
func parseNames(s string) []string {
	var names []string
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func parseMode(modeStr string) evccdb.TransferMode {
	switch modeStr {
	case "config":
		return evccdb.TransferConfig
	case "metrics":
		return evccdb.TransferMetrics
	case "all":
		return evccdb.TransferAll
	default:
		return evccdb.TransferConfig
	}
}
