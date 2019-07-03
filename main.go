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
	Time time.Time `json:"time"`
	Code int       `json:"code"`
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
type SiteDetail struct {
	*SiteStatus
	History []*Status `json:"history"`
}

func checkSiteUpdateStatus(key string, site *Site) {
	log.Printf("[WATCHDOG] [%s] checking status\n", key)

	if resp, err := http.Get(key); err != nil {
		log.Printf("[WATCHDOG] [%s] [ERROR] : %+v\n", key, err)
	} else {
		site.LastCheck = time.Now()
		var encodeSiteErr error
		siteBytes, encodeSiteErr := encodeSite(site)
		if encodeSiteErr != nil {
			log.Printf("[WATCHDOG] [%s] [ERROR] Encoding site : %+v\n", key, encodeSiteErr)
			return
		}
		if err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(sitesBucket))
			if err := b.Put([]byte(key), siteBytes); err != nil {
				log.Printf("[WATCHDOG] [%s] [ERROR] Updating Site LastCheck : %+v\n", key, err)
				return err
			}
			return nil
		}); err != nil {
			log.Printf("[WATCHDOG] [%s] [ERROR] Updating Site LastCheck : %+v\n", key, err)
			return
		}
		if err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(statusBucket))
			statusKey := key + "|" + time.Now().Format(time.RFC3339)
			if err := b.Put([]byte(statusKey), []byte(strconv.Itoa(resp.StatusCode))); err != nil {
				log.Printf("[WATCHDOG] [%s] [ERROR] Put Status : %+v\n", key, err)
				return err
			}
			return nil
		}); err != nil {
			log.Printf("[WATCHDOG] [%s] [ERROR] Put Status : %+v\n", key, err)
			return
		}
		log.Printf("[WATCHDOG] [%s] [STATUS] %d\n", key, resp.StatusCode)
	}
}

func watchDog() {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ticker.C:
			if err := db.View(func(tx *bolt.Tx) error {
				c := tx.Bucket([]byte(sitesBucket)).Cursor()

				for k, v := c.First(); k != nil; k, v = c.Next() {
					site, err := decodeSite(*bytes.NewBuffer(v))
					if err != nil {
						log.Printf("[WATCHDOG] [%s] [ERROR] decoding site: %v\n", k, err)
						continue
					}
					go checkSiteUpdateStatus(string(k), site)
				}
				return nil
			}); err != nil {
				log.Printf("[WATCHDOG] Error reading from db: %v\n", err)
			}
		}
	}
}

func handleGetSite(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["key"]

	if !ok || len(keys[0]) < 1 {
		log.Printf("[ERROR] Invalid QueryString: %+v\n", r.URL.Query())
		http.Error(w, http.StatusText(http.StatusBadRequest)+"\nQuery Param 'key' is missing", http.StatusBadRequest)
		return
	}

	key := keys[0]

	if err := db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(sitesBucket))
		siteBytes := b.Get([]byte(key))
		if siteBytes == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return nil
		}

		siteStatus, siteStatusErr := getSiteStatus(key)
		if siteStatusErr != nil {
			log.Printf("[ERROR] [%s] getting site status: %v\n", key, siteStatusErr)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return nil
		}
		var decodeSiteErr error
		siteStatus.Site, decodeSiteErr = decodeSite(*bytes.NewBuffer(siteBytes))
		if decodeSiteErr != nil {
			log.Printf("[ERROR] [%s] decoding site: %v\n", key, decodeSiteErr)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return nil
		}

		c := tx.Bucket([]byte(statusBucket)).Cursor()
		s := []*Status{}

		prefix := []byte(key + "|")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			t, err := time.Parse(time.RFC3339, strings.TrimPrefix(string(k), string(prefix)))
			if err != nil {
				log.Printf("[ERROR] while parsing time %s: %+v\n", strings.TrimPrefix(string(k), string(prefix)), err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return nil
			}
			code, _ := strconv.Atoi(string(v))
			s = append(s, &Status{
				Time: t,
				Code: code,
			})
		}

		siteDetail := &SiteDetail{
			SiteStatus: siteStatus,
			History:    s,
		}

		if j, err := json.Marshal(siteDetail); err != nil {
			log.Printf("[ERROR] marshalling json %+v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, string(j))
		}
		return nil
	}); err != nil {
		log.Printf("[ERROR] reading from db: %v\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func encodeSite(site *Site) ([]byte, error) {
	var val bytes.Buffer
	enc := gob.NewEncoder(&val)
	if err := enc.Encode(site); err != nil {
		log.Printf("[ERROR] Encoding Site with gob: %v\n", err)
		return nil, err
	}

	return val.Bytes(), nil
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
				log.Printf("[ERROR] while parsing time %s: %+v\n", strings.TrimPrefix(string(k), string(prefix)), err)
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
			siteStatus.Uptime = s[len(s)-1].Time.Sub(s[0].Time).String()
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
		log.Printf("[ERROR] reading from db: %v\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if j, err := json.Marshal(sites); err != nil {
		log.Printf("[ERROR] marshalling json %+v\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, string(j))
	}
}

func handlePostSite(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[ERROR] reading POST body: %v\n", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	payload, perr := url.ParseQuery(string(body))
	if perr != nil {
		log.Printf("[ERROR] parsing body: %v\n", perr)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if payload["key"] == nil {
		log.Printf("[ERROR] Invalid Payload: %+v\n", payload)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	siteBytes, encodeSiteErr := encodeSite(&Site{
		FirstCheck: time.Now(),
		LastCheck:  time.Now(),
	})
	if encodeSiteErr != nil {
		log.Printf("[ERROR] encoding site: %+v\n", encodeSiteErr)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(sitesBucket))
		err := b.Put([]byte(payload["key"][0]), siteBytes)
		return err
	})

	fmt.Fprintf(w, "[ADDED] %s", payload["key"][0])

	log.Printf("Added %s\n", payload["key"][0])
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
		if r.Method == http.MethodGet && len(r.URL.Query()) > 0 {
			handleGetSite(w, r)
		} else if r.Method == http.MethodGet {
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

	go watchDog()

	fmt.Printf("Opened DB %s\nServer listening on port %s\n", dbPath, port)
	http.ListenAndServe(":"+port, nil)
}
