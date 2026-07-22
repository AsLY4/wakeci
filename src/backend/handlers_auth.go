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
	if rejectRateLimited(w, r, logger) {
		return
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
		if recordAuthFailure(w, r, logger) {
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		writeBody(logger, w, []byte("Incorrect password"))
		return
	}
	globalAuthAttemptLimiter.success(authClient(r))

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

	// Set session cookie
	cookie := sessionCookie(r)
	cookie.Value = "delete"
	cookie.Expires = time.Unix(0, 0)
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusNoContent)
}
