package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5"
)

func main() {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	username := os.Getenv("PG_USERNAME")
	password := os.Getenv("PG_PASSWORD")
	database := os.Getenv("PG_DATABASE")
	port := os.Getenv("PG_PORT")
	hostname := os.Getenv("PG_HOSTNAME")

	dbURL := fmt.Sprintf("postgres://%v:%v@%v:%v/%v", username, password, hostname, port, database)

	dbCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := pgx.Connect(dbCtx, dbURL)
	if err != nil {
		log.Error("failed to open connection to postgres")
		os.Exit(1)
	}
	defer conn.Close(dbCtx)

	controller := Controller{conn: conn}

	err = controller.ApplyMigrations()
	if err != nil {
		log.Error("failed to apply migrations", "err", err)
	}

	wholeFile, err := os.ReadFile("ChatExport_2026-03-31/result.json")
	if err != nil {
		log.Error("failed to read file", "err", err)
		os.Exit(1)
	}

	var result Container
	err = json.Unmarshal(wholeFile, &result)
	if err != nil {
		log.Error("failed to unmarshal json", "err", err)
	}

	for _, entry := range result.Messages {
		controller.InsertMessage(entry)
	}
}
