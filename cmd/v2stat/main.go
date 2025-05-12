package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"go.rikki.moe/v2stat/command"
)

var (
	flagServerName  = flag.String("name", "", "Name of the server")
	flagInterval    = flag.Int("interval", 300, "Interval in seconds to record stats")
	flagInflux      = flag.String("influx", "", "URL to InfluxDB database")
	flagInfluxToken = flag.String("token", "", "InfluxDB token")
	flagOrg         = flag.String("org", "", "InfluxDB organization")
	flagBucket      = flag.String("bucket", "", "InfluxDB bucket")
	flagServer      = flag.String("server", "127.0.0.1:8080", "V2Ray API server address")
	flagLogLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error, fatal, panic)")
)

var logger *logrus.Logger

func main() {
	flag.Parse()

	logger = setupLogger(*flagLogLevel)

	// Use hostname as default server name if not provided
	servername := *flagServerName
	if servername == "" {
		hostname, err := os.Hostname()
		if err != nil {
			logger.Fatalf("Failed to get hostname: %v", err)
		}
		servername = hostname
	}
	logger.Infof("Using server name: %s", servername)

	// Set up gRPC connection to V2Ray API server
	conn, err := grpc.NewClient(*flagServer, grpc.WithInsecure())
	if err != nil {
		logger.Fatalf("Failed to connect to V2Ray API server: %v", err)
	}
	defer conn.Close()
	client := command.NewStatsServiceClient(conn)
	if client == nil {
		logger.Fatalf("Failed to create V2Ray API client")
	}

	// Set up InfluxDB client
	influxClient := influxdb2.NewClient(*flagInflux, *flagInfluxToken)
	defer influxClient.Close()

	// Create a new point batch
	bp := influxClient.WriteAPIBlocking(*flagOrg, *flagBucket)

	ticker := time.NewTicker(time.Duration(*flagInterval) * time.Second)
	defer ticker.Stop()
	killsig := make(chan os.Signal, 1)
	signal.Notify(killsig, syscall.SIGINT, syscall.SIGTERM)

	for {
		now := time.Now()
		stats, err := client.QueryStats(context.Background(), &command.QueryStatsRequest{
			Reset_: true,
		})
		if err != nil {
			logger.Errorf("Failed to get stats: %v", err)
			goto LOOP_FINAL
		}

		// Write stats to InfluxDB
		for _, stat := range stats.Stat {
			point := influxdb2.NewPoint(
				"v2ray_stats",
				map[string]string{"server": servername, "stat": stat.Name},
				map[string]interface{}{"value": stat.Value},
				now,
			)
			if err := bp.WritePoint(context.Background(), point); err != nil {
				logger.Errorf("Failed to write point to InfluxDB: %v", err)
			}
		}

LOOP_FINAL:
		select {
		case <-ticker.C:
			continue
		case sig := <-killsig:
			logger.Infof("Received signal: %s", sig)
			return
		}
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
