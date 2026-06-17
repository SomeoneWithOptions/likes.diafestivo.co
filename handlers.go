package main

import (
	"net/http"
	"strconv"
	"strings"

	r "github.com/redis/go-redis/v9"
)

func getLikesHandler(w http.ResponseWriter, req *http.Request) {
	c, err := client.Get(req.Context(), redisKey).Result()
	if err == r.Nil {
		c = "0"
	} else if err != nil {
		http.Error(w, "Failed to read likes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(c))
}

func postLikeHandler(w http.ResponseWriter, req *http.Request) {
	count, err := client.Incr(req.Context(), redisKey).Result()
	if err != nil {
		http.Error(w, "Failed to update likes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.FormatInt(count, 10)))
}

func AuthenticationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}
