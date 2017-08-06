package main

import (
	"flag"
	"log"

	"github.com/prologic/msgbus"
)

var (
	bind string
)

func init() {
	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")
}

func main() {
	log.Fatal(msgbus.NewServer().ListenAndServe(bind))
}
