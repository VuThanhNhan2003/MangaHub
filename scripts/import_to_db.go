package main

import (
	// "encoding/json"
	"log"
	"os"

	"mangahub/pkg/database"
)

func main() {
	log.Println("╔════════════════════════════════════════════════════════╗")
	log.Println("║        Import Manga Data to Database                  ║")
	log.Println("╚════════════════════════════════════════════════════════╝")

	// Check if collection file exists
	if _, err := os.Stat("data/manga_collection.json"); os.IsNotExist(err) {
		log.Println("❌ Error: data/manga_collection.json not found!")
		log.Println("   Please run: make collect-data")
		os.Exit(1)
	}

	// Initialize database
	log.Println("\n📊 Initializing database...")
	db, err := database.InitDB("./data/mangahub.db")
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Clear existing data
	log.Println("🗑️  Clearing existing manga data...")
	_, err = db.Exec("DELETE FROM manga")
	if err != nil {
		log.Fatalf("❌ Failed to clear data: %v", err)
	}

	// Import from JSON
	log.Println("📥 Importing data from manga_collection.json...")
	if err := database.SeedData(db); err != nil {
		log.Fatalf("❌ Failed to import data: %v", err)
	}

	// Count imported manga
	var count int
	db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)

	log.Println("\n╔════════════════════════════════════════════════════════╗")
	log.Printf("║  ✅ Successfully imported %d manga to database        ║\n", count)
	log.Println("╚════════════════════════════════════════════════════════╝")

	// Show sample
	log.Println("\n📚 Sample of imported manga:")
	rows, err := db.Query("SELECT title, author, status FROM manga LIMIT 5")
	if err == nil {
		defer rows.Close()
		i := 1
		for rows.Next() {
			var title, author, status string
			rows.Scan(&title, &author, &status)
			log.Printf("   %d. %s by %s [%s]\n", i, title, author, status)
			i++
		}
	}
}