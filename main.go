package main

import (
	"net/http"
	"os"

	r "github.com/redis/go-redis/v9"
)

var client *r.Client
var requiredAuthToken string
var redisKey string

func init() {

	opt, err := r.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}
	client = r.NewClient(opt)
	requiredAuthToken = os.Getenv("AUTH")
	redisKey = os.Getenv("REDIS_KEY")
}

func main() {
	defer client.Close()
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	http.Handle("GET /", AuthenticationMiddleware(getLikesHandler))
	http.Handle("POST /", AuthenticationMiddleware(postLikeHandler))
	http.ListenAndServe(":"+port, nil)

}
