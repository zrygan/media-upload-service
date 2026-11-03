package main

import (
	"context"
	"flag"
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
	configPath := flag.String("config", ".configs/producer.json", "path to the producer config")
	flag.Parse()

	pc, err := config.ReadConfigFromJSON(*configPath)
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

	workers := worker.MakeWorkers(pc.FolderList)
	slog.Info("producer started", "thread_count", pc.ThreadCount, "consumer_address", pc.ConsumerAddress)
	worker.RunThreads(ctx, workers, client)
	slog.Info("producer stopped")
	return nil
}
