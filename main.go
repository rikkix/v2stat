package main

import (
    "context"
    "database/sql"
    "flag"
    "os"
    "os/signal"
    "time"

    _ "github.com/mattn/go-sqlite3"
    "github.com/sirupsen/logrus"
    "google.golang.org/grpc"

    "go.rikki.moe/v2stat/command"
)

var (
    flagInterval = flag.Int("interval", 300, "Interval in seconds to record stats")
    flagDatabase = flag.String("db", "v2stat.db", "Path to SQLite database")
    flagServer   = flag.String("server", "127.0.0.1:8080", "V2Ray API server address")
    flagLogLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal, panic)")
)

// V2Stat holds references to the logger, database connection, and gRPC client.
type V2Stat struct {
    logger *logrus.Logger
    db     *sql.DB
    stat   command.StatsServiceClient
}

func main() {
    flag.Parse()

    // Initialize logger
    level, err := logrus.ParseLevel(*flagLogLevel)
    if err != nil {
        logrus.Fatalf("Invalid log level: %v", err)
    }
    logger := logrus.New()
    logger.SetLevel(level)

    // Dial gRPC server
    conn, err := grpc.NewClient(*flagServer, grpc.WithInsecure())
    if err != nil {
        logger.Fatalf("Failed to dial gRPC server: %v", err)
    }
    defer conn.Close()

    statClient := command.NewStatsServiceClient(conn)

    // Open SQLite database
    db, err := sql.Open("sqlite3", *flagDatabase)
    if err != nil {
        logger.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    // Create main struct
    v2stat := &V2Stat{
        logger: logger,
        db:     db,
        stat:   statClient,
    }

    // Initialize database schema
    if err := v2stat.InitDB(); err != nil {
        logger.Fatalf("Failed to initialize database: %v", err)
    }

    // For graceful shutdown, create a context that cancels on SIGINT/SIGTERM
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, os.Kill)

    // Optional: Query stats once with a reset (as in your original code)
    if _, err := v2stat.stat.QueryStats(ctx, &command.QueryStatsRequest{Reset_: true}); err != nil {
        logger.Errorf("Failed to query stats: %v", err)
    }

    ticker := time.NewTicker(time.Duration(*flagInterval) * time.Second)
    defer ticker.Stop()
    // Main loop
    for {
        logger.Info("Recording stats...")
        if err := v2stat.RecordNow(ctx); err != nil {
            logger.Errorf("Failed to record stats: %v", err)
        }

        // Wait for next ticker or shutdown signal
        select {
        case <-ticker.C:
            // just continue the loop and record again
        case <-sigCh:
            logger.Info("Received shutdown signal, exiting.")
            return
        case <-ctx.Done():
            logger.Info("Context canceled, exiting.")
            return
        }
    }
}
