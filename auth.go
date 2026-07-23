package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookieName = "nats_monitor_session"

type authConfig struct {
	username     string
	password     string
	sessionTTL   time.Duration
	secureCookie bool
}

type loginAttempt struct {
	windowStart  time.Time
	failures     int
	blockedUntil time.Time
}

type authenticator struct {
	username     string
	usernameHash [sha256.Size]byte
	passwordHash [sha256.Size]byte
	sessionTTL   time.Duration
	secureCookie bool
	secret       [32]byte
	mu           sync.Mutex
	attempts     map[string]loginAttempt
}

func newAuthenticator(cfg authConfig) (*authenticator, error) {
	if cfg.username == "" || cfg.password == "" {
		return nil, errors.New("auth username and password are required")
	}
	a := &authenticator{
		username:     cfg.username,
		usernameHash: sha256.Sum256([]byte(cfg.username)),
		passwordHash: sha256.Sum256([]byte(cfg.password)),
		sessionTTL:   cfg.sessionTTL,
		secureCookie: cfg.secureCookie,
		attempts:     make(map[string]loginAttempt),
	}
	if _, err := rand.Read(a.secret[:]); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *authenticator) credentialsValid(username, password string) bool {
	usernameHash := sha256.Sum256([]byte(username))
	passwordHash := sha256.Sum256([]byte(password))
	usernameOK := subtle.ConstantTimeCompare(usernameHash[:], a.usernameHash[:])
	passwordOK := subtle.ConstantTimeCompare(passwordHash[:], a.passwordHash[:])
	return usernameOK&passwordOK == 1
}

func (a *authenticator) allowLogin(remoteAddr string, now time.Time) bool {
	key := remoteIP(remoteAddr)
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[key]
	return !now.Before(attempt.blockedUntil)
}

func (a *authenticator) recordFailure(remoteAddr string, now time.Time) {
	key := remoteIP(remoteAddr)
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.attempts) > 1024 {
		for address, old := range a.attempts {
			if now.Sub(old.windowStart) >= 15*time.Minute && now.After(old.blockedUntil) {
				delete(a.attempts, address)
			}
		}
	}
	attempt := a.attempts[key]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) >= 5*time.Minute {
		attempt = loginAttempt{windowStart: now}
	}
	attempt.failures++
	if attempt.failures >= 5 {
		attempt.blockedUntil = now.Add(10 * time.Minute)
	}
	a.attempts[key] = attempt
}

func (a *authenticator) clearFailures(remoteAddr string) {
	a.mu.Lock()
	delete(a.attempts, remoteIP(remoteAddr))
	a.mu.Unlock()
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func (a *authenticator) setSession(w http.ResponseWriter, now time.Time) {
	expires := now.Add(a.sessionTTL)
	payload := strconv.FormatInt(expires.Unix(), 10)
	signature := a.sign(payload)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(a.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *authenticator) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *authenticator) isAuthenticated(r *http.Request, now time.Time) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, a.sign(string(payload))) {
		return false
	}
	expires, err := strconv.ParseInt(string(payload), 10, 64)
	return err == nil && now.Unix() < expires
}

func (a *authenticator) sign(payload string) []byte {
	mac := hmac.New(sha256.New, a.secret[:])
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func (a *authenticator) require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.isAuthenticated(r, time.Now()) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}
