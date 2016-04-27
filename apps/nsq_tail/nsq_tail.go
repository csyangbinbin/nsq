package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/absolute8511/go-nsq"
	"github.com/absolute8511/nsq/internal/app"
	"github.com/absolute8511/nsq/internal/version"
)

var (
	showVersion = flag.Bool("version", false, "print version string")

	topic         = flag.String("topic", "", "NSQ topic")
	partition     = flag.Int("partition", -1, "NSQ topic partition")
	channel       = flag.String("channel", "", "NSQ channel")
	maxInFlight   = flag.Int("max-in-flight", 200, "max number of messages to allow in flight")
	totalMessages = flag.Int("n", 0, "total messages to show (will wait if starved)")

	lookupdHTTPAddrs = app.StringArray{}
)

func init() {
	flag.Var(&lookupdHTTPAddrs, "lookupd-http-address", "lookupd HTTP address (may be given multiple times)")
}

type TailHandler struct {
	totalMessages int
	messagesShown int
}

func (th *TailHandler) HandleMessage(m *nsq.Message) error {
	th.messagesShown++
	_, err := os.Stdout.Write(m.Body)
	if err != nil {
		log.Fatalf("ERROR: failed to write to os.Stdout - %s", err)
	}
	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		log.Fatalf("ERROR: failed to write to os.Stdout - %s", err)
	}
	if th.totalMessages > 0 && th.messagesShown >= th.totalMessages {
		os.Exit(0)
	}
	return nil
}

func main() {
	cfg := nsq.NewConfig()
	// TODO: remove, deprecated
	flag.Var(&nsq.ConfigFlag{cfg}, "reader-opt", "(deprecated) use --consumer-opt")
	flag.Var(&nsq.ConfigFlag{cfg}, "consumer-opt", "option to passthrough to nsq.Consumer (may be given multiple times, http://godoc.org/github.com/absolute8511/go-nsq#Config)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("nsq_tail v%s\n", version.Binary)
		return
	}

	if *channel == "" {
		rand.Seed(time.Now().UnixNano())
		*channel = fmt.Sprintf("tail%06d#ephemeral", rand.Int()%999999)
	}

	if *topic == "" {
		log.Fatal("--topic is required")
	}

	if len(lookupdHTTPAddrs) == 0 {
		log.Fatal("--nsqd-tcp-address or --lookupd-http-address required")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Don't ask for more messages than we want
	if *totalMessages > 0 && *totalMessages < *maxInFlight {
		*maxInFlight = *totalMessages
	}

	cfg.UserAgent = fmt.Sprintf("nsq_tail/%s go-nsq/%s", version.Binary, nsq.VERSION)
	cfg.MaxInFlight = *maxInFlight

	consumer, err := nsq.NewPartitionConsumer(*topic, *partition, *channel, cfg)
	if err != nil {
		log.Fatal(err)
	}

	consumer.AddHandler(&TailHandler{totalMessages: *totalMessages})

	err = consumer.ConnectToNSQLookupds(lookupdHTTPAddrs)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case <-consumer.StopChan:
			return
		case <-sigChan:
			consumer.Stop()
		}
	}
}
