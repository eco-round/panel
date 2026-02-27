package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env vars")
	}

	// API Client (replaces direct DB access)
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	api := NewAPIClient(apiURL)

	// Connect to chain (optional — graceful if not configured)
	chain, chainErr := InitChain()
	if chainErr != nil {
		log.Printf("⚠ Chain not connected: %v (on-chain features disabled)\n", chainErr)
	}

	// Launch TUI
	app := NewApp(api)
	app.chain = chain
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
