package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/prologic/msgbus/client"
)

const defaultTopic = "hello"

func main() {
	var (
		host string
		port int
	)

	flag.StringVar(&host, "host", "localhost", "host to connect to")
	flag.IntVar(&port, "port", 8000, "port to connect to")
	flag.Parse()

	client := client.NewClient(host, port, nil)

	if flag.Arg(0) == "sub" {
		subscribe(client, flag.Arg(1))
	} else if flag.Arg(0) == "pub" {
		publish(client, flag.Arg(1), flag.Arg(2))
	} else if flag.Arg(0) == "pull" {
		pull(client, flag.Arg(1))
	} else {
		log.Fatalf("invalid command %s", flag.Arg(0))
	}
}

func publish(client *client.Client, topic, message string) {
	if topic == "" {
		topic = defaultTopic
	}

	if message == "" || message == "-" {
		log.Printf("Reading message from stdin...\n")
		buf, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("error reading message from stdin: %s", err)
		}
		message = string(buf[:])
	}

	err := client.Publish(topic, message)
	if err != nil {
		log.Fatalf("error publishing message: %s", err)
	}
}

func pull(client *client.Client, topic string) {
	if topic == "" {
		topic = defaultTopic
	}

	client.Pull(topic)
}

func subscribe(client *client.Client, topic string) {
	if topic == "" {
		topic = defaultTopic
	}

	s := client.Subscribe(topic)
	go s.Run()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("caught signal %s: ", sig)
		s.Stop()
		done <- true
	}()

	<-done
}
