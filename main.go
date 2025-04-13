package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"go.rikki.moe/v2stat/command"
)

var (
	flagInterval = flag.Int("interval", 300, "Interval in seconds to record stats")
	flagDatabase = flag.String("db", "", "Path to SQLite database")
	flagServer   = flag.String("server", "127.0.0.1:8080", "V2Ray API server address")
	flagLogLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal, panic)")
)

var DefaultDBPaths = []string{
	"v2stat.db",
	"traffic.db",
	"/var/lib/v2stat/traffic.db",
	"/usr/local/share/v2stat/traffic.db",
	"/opt/apps/v2stat/traffic.db",
}

type V2Stat struct {
	logger *logrus.Logger
	db     *sql.DB
	stat   command.StatsServiceClient
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: v2stat <command> [args]")
		fmt.Println("Available commands: daemon, query")
		os.Exit(1)
	}

	logger := setupLogger(*flagLogLevel)
	db := setupDatabase(logger, *flagDatabase)
	defer db.Close()

	v2stat := &V2Stat{
		logger: logger,
		db:     db,
	}

	switch args[0] {
	case "daemon":
		conn, err := grpc.NewClient(*flagServer, grpc.WithInsecure())
		if err != nil {
			logger.Fatalf("Failed to dial gRPC server: %v", err)
		}
		defer conn.Close()
		v2stat.stat = command.NewStatsServiceClient(conn)
		runServer(v2stat)

	case "query":
		handleQuery(v2stat, args[1:])

	default:
		fmt.Println("Unknown command:", args[0])
		fmt.Println("Available commands: daemon, query")
		os.Exit(1)
	}
}

func setupLogger(levelStr string) *logrus.Logger {
	level, err := logrus.ParseLevel(levelStr)
	if err != nil {
		logrus.Fatalf("Invalid log level: %v", err)
	}
	logger := logrus.New()
	logger.SetLevel(level)
	return logger
}

func setupDatabase(logger *logrus.Logger, dbpath string) *sql.DB {
	if dbpath == "" {
		for _, path := range DefaultDBPaths {
			if _, err := os.Stat(path); err == nil {
				dbpath = path
				break
			}
		}
		if dbpath == "" {
			logger.Fatal("No database path provided and no default database found.")
		}
	}
	logger.Infof("Using database: %s", dbpath)

	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		logger.Fatalf("Failed to open database: %v", err)
	}
	return db
}

func runServer(v2stat *V2Stat) {
	logger := v2stat.logger

	if err := v2stat.InitDB(); err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(time.Duration(*flagInterval) * time.Second)
	defer ticker.Stop()

	for {
		logger.Info("Recording stats...")
		if err := v2stat.RecordNow(ctx); err != nil {
			logger.Errorf("Failed to record stats: %v", err)
		}

		select {
		case <-ticker.C:
			continue
		case <-sigCh:
			logger.Info("Received shutdown signal, exiting.")
			return
		case <-ctx.Done():
			logger.Info("Context canceled, exiting.")
			return
		}
	}
}

func handleQuery(v2stat *V2Stat, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: v2stat query <connection_name>")
		fmt.Println("Available connections:")
		conns, err := v2stat.QueryConn()
		if err != nil {
			v2stat.logger.Fatalf("Failed to query connection: %v", err)
		}
		for _, c := range conns {
			fmt.Printf("\t%s\n", c.String())
		}
		return
	}

	connStr := args[0]
	conn, ok := ParseConnInfo(connStr)
	if !ok {
		v2stat.logger.Fatalf("Invalid connection format: %s", connStr)
	}

	stats, err := v2stat.QueryStatsHourly(&conn)
	if err != nil {
		v2stat.logger.Fatalf("Failed to query stats: %v", err)
	}

	printStatsTable(stats)
}

func printStatsTable(stats []TrafficStat) {
	tb := table.NewWriter()
	tb.SetOutputMirror(os.Stdout)
	tb.AppendHeader(table.Row{"Time", "Downlink", "Uplink"})

	var totalDown, totalUp int64

	for _, stat := range stats {
		totalDown += stat.Downlink
		totalUp += stat.Uplink
		tb.AppendRow(table.Row{stat.Time, sizeToHuman(stat.Downlink), sizeToHuman(stat.Uplink)})
	}

	tb.AppendFooter(table.Row{"Total", sizeToHuman(totalDown), sizeToHuman(totalUp)})
	style := table.StyleLight
	style.Format.Footer = text.FormatDefault
	tb.SetStyle(style)
	tb.Render()
}

func sizeToHuman(size int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	for _, unit := range units {
		if value < 1024 {
			return fmt.Sprintf("%7.2f %s", value, unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%7.2f %s", value, "PiB")
}