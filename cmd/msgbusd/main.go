package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prologic/msgbus"
)

var (
	bind string
	ttl  time.Duration
)

func init() {
	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")
	flag.DurationVar(&ttl, "ttl", 60*time.Second, "default ttl")
}

func main() {
	options := msgbus.Options{DefaultTTL: ttl}
	http.Handle("/", msgbus.NewMessageBus(&options))
	log.Fatal(http.ListenAndServe(bind, nil))
}
