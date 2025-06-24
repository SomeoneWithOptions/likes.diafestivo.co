package main

import (
	"net/http"
	"os"
	"strconv"
)

func getLikesHandler(w http.ResponseWriter, r *http.Request) {
	redisKey := os.Getenv("REDIS_KEY")
	auth := os.Getenv("AUTH")
	authHeader := r.Header.Get("Authorization")

	if authHeader != auth {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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
	auth := os.Getenv("AUTH")
	authHeader := r.Header.Get("Authorization")

	if authHeader != auth {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	c := client.Get(r.Context(), redisKey).Val()
	cn, _ := strconv.Atoi(c)
	client.Set(r.Context(), redisKey, cn+1, 0)
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
}
