package main

import (
	"net/http"
	"os"

	r "github.com/redis/go-redis/v9"
)

func init() {

	opt, err := r.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}
	client = r.NewClient(opt)
}

var client *r.Client

func main() {
	defer client.Close()
	port := os.Getenv("PORT")

	if port == "" {
		port = "80"
	}

	http.HandleFunc("GET /", getLikesHandler)
	http.HandleFunc("POST /", postLikeHandler)
	http.ListenAndServe(":"+port, nil)

}
