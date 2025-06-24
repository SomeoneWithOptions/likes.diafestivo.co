package main

import (
	"context"
	"fmt"
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
	redisKey := os.Getenv("REDIS_KEY")
	ctx := context.Background()
	val := client.Get(ctx, redisKey).Val()
	fmt.Println(val)

}
