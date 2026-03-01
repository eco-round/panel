package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env — try cwd first, then the executable's directory
	if err := godotenv.Load(); err != nil {
		if exe, exeErr := os.Executable(); exeErr == nil {
			_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
		}
	}

	// API Client (replaces direct DB access)
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	api := NewAPIClient(apiURL)

	// Connect to chain
	chain, chainErr := InitChain()

	// Launch TUI
	app := NewApp(api)
	app.chain = chain
	app.chainInitErr = chainErr
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
