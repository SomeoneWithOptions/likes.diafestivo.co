package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	redis "github.com/redis/go-redis/v9"
)

const defaultAllowedOrigins = "https://diafestivo.co,https://www.diafestivo.co,https://dev.diafestivo.co"

type config struct {
	RedisURL       string
	RedisKey       string
	AuthToken      string
	Port           string
	AllowedOrigins map[string]struct{}
}

type redisStore interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
}

type app struct {
	store  redisStore
	cfg    config
	logger *slog.Logger
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		RedisURL:  strings.TrimSpace(getenv("REDIS_URL")),
		RedisKey:  strings.TrimSpace(getenv("REDIS_KEY")),
		AuthToken: strings.TrimSpace(getenv("AUTH")),
		Port:      strings.TrimSpace(getenv("PORT")),
	}

	if cfg.RedisURL == "" {
		return config{}, errors.New("REDIS_URL is required")
	}
	if cfg.RedisKey == "" {
		return config{}, errors.New("REDIS_KEY is required")
	}
	if cfg.AuthToken == "" {
		return config{}, errors.New("AUTH is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if _, err := strconv.ParseUint(cfg.Port, 10, 16); err != nil {
		return config{}, fmt.Errorf("PORT must be a valid TCP port: %w", err)
	}

	allowedOrigins := strings.TrimSpace(getenv("ALLOWED_ORIGINS"))
	if allowedOrigins == "" {
		allowedOrigins = defaultAllowedOrigins
	}

	origins, err := parseAllowedOrigins(allowedOrigins)
	if err != nil {
		return config{}, err
	}
	cfg.AllowedOrigins = origins

	return cfg, nil
}

func parseAllowedOrigins(raw string) (map[string]struct{}, error) {
	origins := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		origin, ok := normalizeOrigin(part)
		if !ok {
			return nil, fmt.Errorf("invalid allowed origin %q", strings.TrimSpace(part))
		}
		origins[origin] = struct{}{}
	}
	if len(origins) == 0 {
		return nil, errors.New("ALLOWED_ORIGINS must include at least one origin")
	}
	return origins, nil
}

func normalizeOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	if u.Path != "" && u.Path != "/" {
		return "", false
	}

	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), true
}

func newApp(store redisStore, cfg config, logger *slog.Logger) *app {
	return &app{store: store, cfg: cfg, logger: logger}
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.authMiddleware(a.getLikesHandler))
	mux.HandleFunc("POST /", a.authMiddleware(a.postLikeHandler))
	mux.HandleFunc("OPTIONS /", a.preflightHandler)
	return mux
}

func (a *app) getLikesHandler(w http.ResponseWriter, req *http.Request) {
	count, err := a.store.Get(req.Context(), a.cfg.RedisKey).Result()
	if errors.Is(err, redis.Nil) {
		count = "0"
	} else if err != nil {
		a.logger.Error("failed to read likes", "error", err)
		http.Error(w, "Failed to read likes", http.StatusInternalServerError)
		return
	}

	setTextHeaders(w)
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, count); err != nil {
		a.logger.Error("failed to write response", "error", err)
	}
}

func (a *app) postLikeHandler(w http.ResponseWriter, req *http.Request) {
	count, err := a.store.Incr(req.Context(), a.cfg.RedisKey).Result()
	if err != nil {
		a.logger.Error("failed to update likes", "error", err)
		http.Error(w, "Failed to update likes", http.StatusInternalServerError)
		return
	}

	setTextHeaders(w)
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, count); err != nil {
		a.logger.Error("failed to write response", "error", err)
	}
}

func (a *app) preflightHandler(w http.ResponseWriter, req *http.Request) {
	if !a.setCORSHeaders(w, req.Header.Get("Origin")) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		originAllowed := a.setCORSHeaders(w, req.Header.Get("Origin"))
		authenticated := secureCompare(req.Header.Get("Authorization"), a.cfg.AuthToken)

		if !originAllowed && !authenticated {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, req)
	}
}

func (a *app) setCORSHeaders(w http.ResponseWriter, originHeader string) bool {
	origin, ok := normalizeOrigin(originHeader)
	if !ok {
		return false
	}
	if _, ok := a.cfg.AllowedOrigins[origin]; !ok {
		return false
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Add("Vary", "Origin")
	return true
}

func setTextHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
}

func secureCompare(got, want string) bool {
	if got == "" || want == "" {
		return false
	}

	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}
