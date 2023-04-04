package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/go-sql-driver/mysql"
)

const (
	baseURL    = "http://localhost:8080/"
	expireTime = 24 * time.Hour
)

type ShortLink struct {
	ID           int       `json:"id"`
	OriginalLink string    `json:"original_link"`
	ShortLink    string    `json:"short_link"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiryAt     time.Time `json:"expiry_at"`
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/shorten", shortenLink).Methods("POST")
	router.HandleFunc("/{shortLink}", redirectToLink).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}

func shortenLink(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/urlshortener")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var originalLink string
	err = json.NewDecoder(r.Body).Decode(&originalLink)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the original link already exists in the database
	var shortLink string
	err = db.QueryRow("SELECT short_link FROM short_links WHERE original_link = ?", originalLink).Scan(&shortLink)
	if err == nil {
		// If the original link already exists, return the existing short link
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"short_link": baseURL + shortLink,
			"expiry_at":  time.Now().Add(expireTime).Format(time.RFC3339),
		})
		return
	}

	// Generate a unique short link for the original link
	hash := sha256.Sum256([]byte(originalLink))
	shortLinkBytes := base64.RawURLEncoding.EncodeToString(hash[:])[:8]
	shortLink = string(shortLinkBytes)

	// Insert the new short link into the database
	stmt, err := db.Prepare("INSERT INTO short_links (original_link, short_link, created_at, expiry_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(originalLink, shortLink, time.Now(), time.Now().Add(expireTime))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the new short link
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"short_link": baseURL + shortLink,
		"expiry_at":  time.Now().Add(expireTime).Format(time.RFC3339),
	})
}

func redirectToLink(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/urlshortener")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	shortLink := mux.Vars(r)["shortLink"]

	// Retrieve the original link from the database
	var originalLink string
	var expiryAt
