// Command admin provides local owner-only recovery operations.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	authapp "sre-kit/internal/auth/application"
	"sre-kit/internal/platform/config"
	"sre-kit/internal/platform/secrets"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "rotate-password" {
		log.Fatal("usage: go run ./cmd/admin rotate-password")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	store, err := secrets.Open(cfg.SecretsPath, cfg.SecretsKey)
	if err != nil {
		log.Fatal(err)
	}
	password, err := authapp.NewService(store).RotateAdminPassword(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("new admin password (save it now): %s\n", password)
}
