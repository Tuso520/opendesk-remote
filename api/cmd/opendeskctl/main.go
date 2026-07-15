package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
	"github.com/opendesk-remote/opendesk-remote/api/internal/database"
	"github.com/opendesk-remote/opendesk-remote/api/internal/migrations"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: opendeskctl migrate")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "migrate":
		if err := migrate(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		os.Exit(2)
	}
}

func migrate() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("OPENDESK_MYSQL_DSN is required for migrations")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := database.Ping(ctx, db); err != nil {
		return err
	}
	if err := migrations.Apply(ctx, db, "migrations"); err != nil {
		return err
	}
	fmt.Println("migrations applied")
	return nil
}
