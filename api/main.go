package main

import (
	"context"
	"log/slog"
	"os"

	"workflow-code-test/api/builder"
)

func main() {
	// Create a context for the application
	ctx := context.Background()

	// Build the application using the builder package
	app, err := builder.Build(ctx)
	if err != nil {
		// If we can't build the app, log and exit
		slog.Error("Failed to build application", "error", err)
		os.Exit(1)
	}

	// Ensure resources are cleaned up on exit
	defer app.Close()

	// Run the application (this handles the server and graceful shutdown)
	if err := app.Run(ctx); err != nil {
		app.Logger.Error("Application stopped with error", "error", err)
		os.Exit(1)
	}
}
