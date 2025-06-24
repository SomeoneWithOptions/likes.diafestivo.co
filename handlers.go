package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

func getLikesHandler(w http.ResponseWriter, r *http.Request) {
	redisKey := os.Getenv("REDIS_KEY")

	c := client.Get(r.Context(), redisKey).Val()
	if c == "" {
		c = "0"
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(c))
}

func postLikeHandler(w http.ResponseWriter, r *http.Request) {
	redisKey := os.Getenv("REDIS_KEY")

	c := client.Get(r.Context(), redisKey).Val()
	cn, _ := strconv.Atoi(c)
	client.Set(r.Context(), redisKey, cn+1, 0)

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
}

func AuthenticationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAllowedOrigin := func(r *http.Request) bool {
			return strings.Contains(r.Header.Get("Origin"), "diafestivo.co")
		}
		isAuthenticated := func(r *http.Request) bool {
			return requiredAuthToken != "" && r.Header.Get("Authorization") == requiredAuthToken
		}

		if isAllowedOrigin(r) || isAuthenticated(r) {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}
