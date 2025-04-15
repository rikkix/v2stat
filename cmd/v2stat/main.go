package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"go.rikki.moe/v2stat"
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

var logger *logrus.Logger

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: v2stat <command> [args]")
		fmt.Println("Available commands: daemon, query")
		os.Exit(1)
	}

	logger = setupLogger(*flagLogLevel)
	db := setupDatabase(logger, *flagDatabase)

	v2s := v2stat.NewV2Stat(logger, db, nil)
	defer v2s.Close()

	switch args[0] {
	case "daemon":
		conn, err := grpc.NewClient(*flagServer, grpc.WithInsecure())
		if err != nil {
			logger.Fatalf("Failed to dial gRPC server: %v", err)
		}
		v2s.SetConn(conn)
		runDaemon(v2s)

	case "query":
		handleQuery(v2s, args[1:])

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

func handleQuery(v2s *v2stat.V2Stat, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: v2stat query <connection_name>")
		fmt.Println("Available connections:")
		conns, err := v2s.QueryConn()
		if err != nil {
			logger.Fatalf("Failed to query connection: %v", err)
		}
		for _, c := range conns {
			fmt.Printf("\t%s\n", c.String())
		}
		return
	}

	connStr := args[0]
	conn, ok := v2stat.ParseConnInfo(connStr)
	if !ok {
		logger.Fatalf("Invalid connection format: %s", connStr)
	}

	stats, err := v2s.QueryStatsHourly(&conn)
	if err != nil {
		logger.Fatalf("Failed to query stats: %v", err)
	}

	printStatsTable(stats)
}

func printStatsTable(stats []v2stat.TrafficStat) {
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
