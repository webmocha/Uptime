package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	defaultDbPath    = "uptime.db"
	sitesBucketName  = "Sites"
	statusBucketName = "Status"
	staticDir        = "static/"
	portEnv          = "PORT"
	defaultPort      = "80"
)

var (
	db           *bolt.DB
	sitesBucket  *bolt.Bucket
	statusBucket *bolt.Bucket
)

func handleGetSites(w http.ResponseWriter, r *http.Request) {

}

func handlePostSite(w http.ResponseWriter, r *http.Request) {

}

func main() {
	dbPath := defaultDbPath
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	var err error
	db, err = bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		var err error
		sitesBucket, err = tx.CreateBucketIfNotExists([]byte(sitesBucketName))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", sitesBucketName, err)
		}
		return nil
	})

	db.Update(func(tx *bolt.Tx) error {
		var err error
		statusBucket, err = tx.CreateBucketIfNotExists([]byte(statusBucketName))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", statusBucketName, err)
		}
		return nil
	})

	http.HandleFunc("/api/sites", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleGetSites(w, r)
		} else if r.Method == http.MethodPost {
			handlePostSite(w, r)
		} else {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/", http.StripPrefix("/"+staticDir, fs))

	port := os.Getenv(portEnv)
	if port == "" {
		port = defaultPort
	}

	fmt.Printf("Opened DB %s\nServer listening on port %s", dbPath, port)
	http.ListenAndServe(":"+port, nil)
}
