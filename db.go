package main

import (
	"context"
	"strings"
	"time"

	"go.rikki.moe/v2stat/command"
)

const (
	DirectionDownlink = iota
	DirectionUplink
)

const (
	ConnTypeUser = iota
	ConnTypeInbound
	ConnTypeOutbound
)

type ConnInfo struct {
	Type     int    `json:"type"`
	Name     string `json:"name"`
}

func (v *V2Stat) InitDB() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conn (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type INTEGER NOT NULL,
			name TEXT NOT NULL,
			UNIQUE (type, name)
		);`,
		`CREATE TABLE IF NOT EXISTS stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conn_id INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			traffic INTEGER NOT NULL,
			direction INTEGER NOT NULL,
			FOREIGN KEY (conn_id) REFERENCES conn (id)
				ON DELETE CASCADE
				ON UPDATE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_conn_id ON stats (conn_id);`,
		`CREATE INDEX IF NOT EXISTS idx_timestamp ON stats (timestamp);`,
	}

	for _, stmt := range stmts {
		if _, err := v.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (v *V2Stat) RecordNow(ctx context.Context) error {
	resp, err := v.stat.QueryStats(ctx, &command.QueryStatsRequest{
		Reset_: true,
	})
	if err != nil {
		v.logger.Errorf("Failed to query stats: %v", err)
		return err
	}

	tx, err := v.db.Begin()
	if err != nil {
		v.logger.Errorf("Failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback() // safe to call even if already committed

	insertConnStmt, err := tx.Prepare(`INSERT OR IGNORE INTO conn (type, name) VALUES (?, ?)`)
	if err != nil {
		v.logger.Errorf("Failed to prepare conn insert statement: %v", err)
		return err
	}
	defer insertConnStmt.Close()

	selectConnIDStmt, err := tx.Prepare(`SELECT id FROM conn WHERE type = ? AND name = ?`)
	if err != nil {
		v.logger.Errorf("Failed to prepare conn select statement: %v", err)
		return err
	}
	defer selectConnIDStmt.Close()

	insertStatsStmt, err := tx.Prepare(`
		INSERT INTO stats (conn_id, timestamp, traffic, direction)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		v.logger.Errorf("Failed to prepare stats insert statement: %v", err)
		return err
	}
	defer insertStatsStmt.Close()

	for _, stat := range resp.Stat {
		connType, connName, direction, ok := parseStatKey(stat.Name)
		if !ok {
			v.logger.Warnf("Skipping unrecognized stat key: %s", stat.Name)
			continue
		}

		if _, err := insertConnStmt.Exec(connType, connName); err != nil {
			v.logger.Errorf("Failed to insert conn: %v", err)
			continue
		}

		var connID int
		err = selectConnIDStmt.QueryRow(connType, connName).Scan(&connID)
		if err != nil {
			v.logger.Errorf("Failed to retrieve conn_id: %v", err)
			continue
		}

		timeNow := time.Now().Unix()
		if _, err := insertStatsStmt.Exec(connID, timeNow, stat.Value, direction); err != nil {
			v.logger.Errorf("Failed to insert stats: %v", err)
			continue
		}

		v.logger.Infof("Inserted stats: conn_id=%d, timestamp=%d, traffic=%d, direction=%d", connID, timeNow, stat.Value, direction)
	}

	if err := tx.Commit(); err != nil {
		v.logger.Errorf("Failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func parseStatKey(key string) (connType int, connName string, direction int, ok bool) {
	parts := strings.Split(key, ">>>")
	if len(parts) != 4 || parts[2] != "traffic" {
		return 0, "", 0, false
	}

	switch parts[0] {
	case "user":
		connType = ConnTypeUser
	case "inbound":
		connType = ConnTypeInbound
	case "outbound":
		connType = ConnTypeOutbound
	default:
		return 0, "", 0, false
	}

	connName = parts[1]

	switch parts[3] {
	case "downlink":
		direction = DirectionDownlink
	case "uplink":
		direction = DirectionUplink
	default:
		return 0, "", 0, false
	}

	return connType, connName, direction, true
}