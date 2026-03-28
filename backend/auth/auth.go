package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Simple in-memory session store
var (
	sessions = make(map[string]session)
	mutex    = &sync.Mutex{}
)

type session struct {
	username string
	expiry   time.Time
}

func (s session) isExpired() bool {
	return s.expiry.Before(time.Now())
}

const (
	hardcodedUsername       = "admin"
	hardcodedHashedPassword = "29e0394f" // FNV-1a hash of "password"
	SessionCookieName       = "session_token"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

// Login checks the credentials and creates a session if they are valid.
// It returns the session token and an error if login fails.
func Login(username, hashedPassword string) (string, error) {
	// In a real application, you would look up the user in a database
	// and compare the hashed password.
	if username == hardcodedUsername && hashedPassword == hardcodedHashedPassword {
		// Credentials are valid, create a new session
		sessionToken := make([]byte, 32)
		if _, err := rand.Read(sessionToken); err != nil {
			return "", err
		}
		tokenString := hex.EncodeToString(sessionToken)

		mutex.Lock()
		sessions[tokenString] = session{
			username: username,
			expiry:   time.Now().Add(24 * time.Hour), // Session expires in 24 hours
		}
		mutex.Unlock()

		return tokenString, nil
	}
	return "", ErrInvalidCredentials
}

// Logout removes a session.
func Logout(token string) {
	mutex.Lock()
	delete(sessions, token)
	mutex.Unlock()
}

// IsAuthenticated checks if a request is authenticated.
func IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return false
	}
	sessionToken := cookie.Value

	mutex.Lock()
	userSession, exists := sessions[sessionToken]
	mutex.Unlock()

	if !exists {
		return false
	}

	if userSession.isExpired() {
		Logout(sessionToken)
		return false
	}

	return true
}

// SetSessionCookie sets the session cookie in the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true, // Important for security
		Path:     "/",
	})
}

// ClearSessionCookie clears the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	})
}
