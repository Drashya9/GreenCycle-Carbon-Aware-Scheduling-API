// Command migrate applies every pending migration and exits. Use it where the app itself
// runs with RUN_MIGRATIONS=false (e.g. Vercel), once per schema change:
//
//	DATABASE_URL_UNPOOLED=postgres://... go run ./cmd/migrate
//
// It prefers DATABASE_URL_UNPOOLED over DATABASE_URL because golang-migrate takes a
// session-level advisory lock, which a transaction-mode pooler (e.g. Neon's) can't honor.
package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/config"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/db"
)

func main() {
	_ = godotenv.Load()

	url := os.Getenv("DATABASE_URL_UNPOOLED")
	if url == "" {
		url = config.Load().DatabaseURL
	}

	dbx, err := db.Connect(url)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer dbx.Close()

	if err := db.Migrate(dbx); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("migrations up to date")
}
