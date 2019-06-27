package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	defaultDbPath = "uptime.db"
	sitesBucket   = "Sites"
	statusBucket  = "Status"
	staticDir     = "static/"
	portEnv       = "PORT"
	defaultPort   = "80"
)

var (
	db *bolt.DB
)

type Status struct {
	Time time.Time
	Code int
}
type Sites []*SiteStatus
type Site struct {
	FirstCheck time.Time `json:"firstCheck"`
	LastCheck  time.Time `json:"lastCheck"`
}
type SiteStatus struct {
	*Site
	Key        string `json:"key"`
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Uptime     string `json:"uptime"`
}

func handleGetSite(w http.ResponseWriter, r *http.Request) {
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(sitesBucket))
		v := b.Get([]byte("answer"))

		fmt.Printf("The answer is: %s\n", v)
		return nil
	})
	if err != nil {
		log.Printf("Error reading from db: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func decodeSite(buf bytes.Buffer) (*Site, error) {
	site := &Site{}
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(site)
	return site, err
}

func getSiteStatus(key string) (*SiteStatus, error) {
	siteStatus := &SiteStatus{
		Key: key,
	}
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(statusBucket)).Cursor()
		s := []*Status{}

		prefix := []byte(key + "|")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			t, err := time.Parse(time.RFC3339, strings.TrimPrefix(string(k), string(prefix)))
			if err != nil {
				log.Printf("Error while parsing time %s: %+v", strings.TrimPrefix(string(k), string(prefix)), err)
				return err
			}
			code, _ := strconv.Atoi(string(v))
			s = append(s, &Status{
				Time: t,
				Code: code,
			})
		}

		var first *Status
		uptimes := []time.Time{}
		for _, v := range s {
			if v.Code < http.StatusBadRequest {
				if first == nil {
					first = v
				}
				uptimes = append(uptimes, v.Time)
			} else {
				first = nil
				uptimes = []time.Time{}
			}
		}

		if len(s) > 0 {
			siteStatus.Status = s[len(s)-1].Code
			siteStatus.StatusText = http.StatusText(siteStatus.Status)
			siteStatus.Uptime = s[0].Time.Sub(s[len(s)-1].Time).String()
		} else {
			siteStatus.Uptime = "0s"
		}

		return nil
	})

	return siteStatus, err
}

func handleGetSites(w http.ResponseWriter, r *http.Request) {
	sites := Sites{}
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(sitesBucket)).Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			siteStatus, siteStatusErr := getSiteStatus(string(k))
			if siteStatusErr != nil {
				return siteStatusErr
			}
			var decodeSiteErr error
			siteStatus.Site, decodeSiteErr = decodeSite(*bytes.NewBuffer(v))
			if decodeSiteErr != nil {
				return decodeSiteErr
			}
			sites = append(sites, siteStatus)
		}

		return nil
	})
	if err != nil {
		log.Printf("Error reading from db: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if j, err := json.Marshal(sites); err != nil {
		log.Printf("Error marshalling json %+v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else {
		fmt.Fprintf(w, string(j))
	}
}

func handlePostSite(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading POST body: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	payload, perr := url.ParseQuery(string(body))
	if perr != nil {
		log.Printf("Error parsing body: %v", perr)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if payload["key"] == nil {
		log.Printf("Error Invalid Payload: %+v", payload)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var val bytes.Buffer
	enc := gob.NewEncoder(&val)
	goberr := enc.Encode(Site{
		FirstCheck: time.Now(),
		LastCheck:  time.Now(),
	})
	if goberr != nil {
		log.Printf("Error Encoding Site with gob: %v", goberr)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(sitesBucket))
		err := b.Put([]byte(payload["key"][0]), val.Bytes())
		return err
	})

	fmt.Fprintf(w, "Added %s", payload["key"][0])

	log.Printf("Added %s", payload["key"][0])
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
		_, err = tx.CreateBucketIfNotExists([]byte(sitesBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", sitesBucket, err)
		}
		return nil
	})

	db.Update(func(tx *bolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists([]byte(statusBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", statusBucket, err)
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
