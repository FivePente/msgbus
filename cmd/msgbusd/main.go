package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prologic/msgbus"
)

func main() {

	var (
		version bool
		bind    string
		ttl     time.Duration
	)

	flag.BoolVar(&version, "v", false, "display version information")

	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")
	flag.DurationVar(&ttl, "ttl", 60*time.Second, "default ttl")

	flag.Parse()

	if version {
		fmt.Printf("msgbusd %s", msgbus.FullVersion())
		os.Exit(0)
	}

	options := msgbus.Options{DefaultTTL: ttl}
	http.Handle("/", msgbus.NewMessageBus(&options))
	log.Printf("msgbusd %s listening on %s", msgbus.FullVersion(), bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}
