package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	redis "github.com/redis/go-redis/v9"
)

type fakeStore struct {
	mu      sync.Mutex
	value   int64
	exists  bool
	getErr  error
	incrErr error
}

func (s *fakeStore) Get(_ context.Context, _ string) *redis.StringCmd {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.getErr != nil {
		return redis.NewStringResult("", s.getErr)
	}
	if !s.exists {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(strconv.FormatInt(s.value, 10), nil)
}

func (s *fakeStore) Incr(_ context.Context, _ string) *redis.IntCmd {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.incrErr != nil {
		return redis.NewIntResult(0, s.incrErr)
	}
	s.value++
	s.exists = true
	return redis.NewIntResult(s.value, nil)
}

func testApp(store *fakeStore) *app {
	return newApp(store, config{
		RedisKey:  "likes",
		AuthToken: "secret",
		AllowedOrigins: map[string]struct{}{
			"https://diafestivo.co": {},
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGetLikesMissingKeyReturnsZero(t *testing.T) {
	app := testApp(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://diafestivo.co")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body != "0" {
		t.Fatalf("body = %q, want %q", body, "0")
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://diafestivo.co" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestPostLikeIncrements(t *testing.T) {
	app := testApp(&fakeStore{})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://diafestivo.co")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body != "1" {
		t.Fatalf("body = %q, want %q", body, "1")
	}
}

func TestAuthTokenAllowsRequestWithoutOrigin(t *testing.T) {
	app := testApp(&fakeStore{value: 7, exists: true})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "secret")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body != "7" {
		t.Fatalf("body = %q, want %q", body, "7")
	}
}

func TestRejectsMissingAuthAndOrigin(t *testing.T) {
	app := testApp(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestRejectsMaliciousOriginSubstring(t *testing.T) {
	app := testApp(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://diafestivo.co.evil.example")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestPreflightAllowedOrigin(t *testing.T) {
	app := testApp(&fakeStore{})
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://diafestivo.co")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if got := res.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://diafestivo.co" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestRedisErrorsReturnServerError(t *testing.T) {
	app := testApp(&fakeStore{getErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://diafestivo.co")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://diafestivo.co" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestLoadConfig(t *testing.T) {
	env := map[string]string{
		"REDIS_URL":       "redis://localhost:6379/0",
		"REDIS_KEY":       "likes",
		"AUTH":            "secret",
		"ALLOWED_ORIGINS": "https://diafestivo.co, https://www.diafestivo.co",
	}

	cfg, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
	}
	if _, ok := cfg.AllowedOrigins["https://www.diafestivo.co"]; !ok {
		t.Fatal("missing allowed origin")
	}
}

func TestLoadConfigDefaultAllowedOriginsIncludesDev(t *testing.T) {
	env := map[string]string{
		"REDIS_URL": "redis://localhost:6379/0",
		"REDIS_KEY": "likes",
		"AUTH":      "secret",
	}

	cfg, err := loadConfig(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if _, ok := cfg.AllowedOrigins["https://dev.diafestivo.co"]; !ok {
		t.Fatal("missing dev allowed origin")
	}
}

func TestLoadConfigRequiresAuth(t *testing.T) {
	env := map[string]string{
		"REDIS_URL": "redis://localhost:6379/0",
		"REDIS_KEY": "likes",
	}

	_, err := loadConfig(func(key string) string { return env[key] })
	if err == nil {
		t.Fatal("loadConfig returned nil error")
	}
}
