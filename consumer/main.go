package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zrygan/media-upload-service/consumer/config"
	"github.com/zrygan/media-upload-service/consumer/queue"
	"github.com/zrygan/media-upload-service/util"
)

func main() {
	util.SetupLogging()

	if err := run(); err != nil {
		slog.Error("consumer failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", ".configs/consumer.json", "path to the consumer config")
	flag.Parse()

	cc, err := config.ReadConfigFromJSON(*configPath)
	if err != nil {
		return err
	}

	q := queue.New(cc.QueueMaxSize)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("consumer started", "queue_max_size", cc.QueueMaxSize, "thread_count", cc.ThreadCount, "listen_address", cc.ListenAddress, "media_folder", cc.MediaFolder, "staging_folder", cc.StagingFolder)

	slog.Info("consumer stopped")
	return nil
}
