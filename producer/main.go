package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zrygan/media-upload-service/producer/config"
	"github.com/zrygan/media-upload-service/producer/network"
	"github.com/zrygan/media-upload-service/producer/worker"
	"github.com/zrygan/media-upload-service/util"
)

func main() {
	util.SetupLogging()

	if err := run(); err != nil {
		slog.Error("producer failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	pc, err := config.ReadConfigFromJSON(".configs/producer.json")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := network.New(pc.ConsumerAddress)
	if err != nil {
		return err
	}
	defer client.Close()
	workers := worker.MakeWorkers(*pc)
	slog.Info("producer started", "thread_count", pc.ThreadCount, "consumer_address", pc.ConsumerAddress)
	worker.RunThreads(workers, *client, ctx)
	slog.Info("producer stopped")
	return nil
}
