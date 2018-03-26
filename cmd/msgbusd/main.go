package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/msgbus"
)

func main() {

	var (
		version bool
		debug   bool
		bind    string
		ttl     time.Duration
	)

	flag.BoolVar(&version, "v", false, "display version information")
	flag.BoolVar(&debug, "d", false, "enable debug logging")

	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")
	flag.DurationVar(&ttl, "ttl", 60*time.Second, "default ttl")

	flag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if version {
		fmt.Printf("msgbusd %s", msgbus.FullVersion())
		os.Exit(0)
	}

	options := msgbus.Options{DefaultTTL: ttl}
	http.Handle("/", msgbus.NewMessageBus(&options))
	log.Infof("msgbusd %s listening on %s", msgbus.FullVersion(), bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}
