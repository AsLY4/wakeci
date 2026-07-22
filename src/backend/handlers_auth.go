package main

import (
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	bolt "go.etcd.io/bbolt"
)

// HandleIsLoggedIn returns 200 if user is logged in
func HandleIsLoggedIn(w http.ResponseWriter, r *http.Request) {
	// See AuthMi
}

// HandleLogIn verifies password and logs the user in
func HandleLogIn(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*slog.Logger)
	if !ok {
		logger = L
	}

	// Create and store session token
	password := r.FormValue("password")

	var hashedPassword []byte

	err := DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GlobalBucket))
		hashedPassword = b.Get([]byte("password"))
		return nil
	})
	if err != nil {
		logger.Error("read password hash", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		// Never log the submitted password.
		logger.Warn("login failed", "err", err)
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "text/plain")
		writeBody(logger, w, []byte("Incorrect password"))
		return
	}

	c, err := GlobalSessionStorage.New(r)
	if err != nil {
		logger.Error("create session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Set session cookie
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}

// HandleLogOut logs the user out
func HandleLogOut(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*slog.Logger)
	if !ok {
		logger = L
	}

	sessionToken, err := r.Cookie("session")
	if err == nil {
		err = GlobalSessionStorage.Delete(sessionToken.Value)
		if err != nil {
			logger.Error("delete session", "err", err)
		}
	}

	expires, _ := time.Parse(time.RFC3339, "1970-01-01T00:00:00+00:00")

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "session",
		Value:   "delete",
		Expires: expires,
		Path:    "/",
	})
	w.WriteHeader(http.StatusNoContent)
}
