package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"time"
)

type targetRequest struct {
	URL string `json:"url"`
}

func newHTTPHandler(c *collector, auth *authenticator, assets fs.FS, targets []monitorTarget) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if auth.isAuthenticated(r, time.Now()) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		http.ServeFileFS(w, r, assets, "login.html")
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		if !auth.allowLogin(r.RemoteAddr, now) {
			http.Redirect(w, r, "/login?error=blocked", http.StatusSeeOther)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if err := r.ParseForm(); err != nil || !auth.credentialsValid(r.FormValue("username"), r.FormValue("password")) {
			auth.recordFailure(r.RemoteAddr, now)
			http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
			return
		}
		auth.clearFailures(r.RemoteAddr)
		auth.setSession(w, now)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.Handle("POST /logout", auth.require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.clearSession(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})))
	mux.Handle("GET /api/snapshot", auth.require(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		value := c.snapshot()
		value.Viewer = auth.username
		value.Targets = append([]monitorTarget(nil), targets...)
		writeJSON(w, http.StatusOK, value)
	})))
	mux.Handle("POST /api/target", auth.require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var request targetRequest
		if err := decoder.Decode(&request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "请求格式无效")
			return
		}
		target, err := c.switchTarget(r.Context(), request.URL)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"target": target})
	})))
	mux.Handle("GET /", auth.require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, assets, "index.html")
	})))
	return securityHeaders(mux)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}
