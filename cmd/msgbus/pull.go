package main

import (
	"github.com/spf13/cobra"

	"github.com/prologic/msgbus/client"
)

// pullCmd represents the pub command
var pullCmd = &cobra.Command{
	Use:   "pull [flags] <topic>",
	Short: "Pulls a message from a given topic",
	Long: `This pulls a message from the given topic if there are any messages
and prints the message to standard output. Otherwise if the queue for the
given topic is empty, this does nothing.

This is primarily useful in situations where a subscription was lost and you
want to "catch up" and pull any messages left in the queue for that topic.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uri := cmd.Flag("uri").Value.String()
		client := client.NewClient(uri, nil)

		topic := args[0]

		pull(client, topic)
	},
}

func init() {
	RootCmd.AddCommand(pullCmd)
}

func pull(client *client.Client, topic string) {
	if topic == "" {
		topic = defaultTopic
	}

	client.Pull(topic)
}
