package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prologic/msgbus"
)

var (
	bind string
)

func init() {
	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")
}

func main() {
	http.Handle("/", msgbus.NewMessageBus())
	log.Fatal(http.ListenAndServe(bind, nil))
}
