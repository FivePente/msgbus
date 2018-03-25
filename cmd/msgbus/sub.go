package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/prologic/msgbus/client"
)

// subCmd represents the pub command
var subCmd = &cobra.Command{
	Use:   "sub [flags] <topic>",
	Short: "Subscribe to a topic",
	Long: `This subscribes to the given topic and for every message published
to the topic, the message is printed to standard output.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		client := client.NewClient(uri, nil)

		topic := args[0]

		subscribe(client, topic)
	},
}

func init() {
	RootCmd.AddCommand(subCmd)
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
