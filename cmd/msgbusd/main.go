package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/msgbus"
)

func main() {
	var (
		version        bool
		debug          bool
		bind           string
		bufferLength   int
		maxQueueSize   int
		maxPayloadSize int
	)

	flag.BoolVar(&version, "v", false, "display version information")
	flag.BoolVar(&debug, "d", false, "enable debug logging")

	flag.StringVar(&bind, "bind", ":8000", "interface and port to bind to")

	flag.IntVar(&bufferLength, "buffer-length", msgbus.DefaultBufferLength, "buffer length")
	flag.IntVar(&maxQueueSize, "max-queue-size", msgbus.DefaultMaxQueueSize, "maximum queue size")
	flag.IntVar(&maxPayloadSize, "max-payload-size", msgbus.DefaultMaxPayloadSize, "maximum payload size")

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

	opts := msgbus.Options{
		BufferLength:   bufferLength,
		MaxQueueSize:   maxQueueSize,
		MaxPayloadSize: maxPayloadSize,
		WithMetrics:    true,
	}
	mb := msgbus.New(&opts)

	http.Handle("/", mb)
	http.Handle("/metrics", mb.Metrics().Handler())
	log.Infof("msgbusd %s listening on %s", msgbus.FullVersion(), bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}
