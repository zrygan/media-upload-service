package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/zrygan/media-upload-service/consumer/config"
	"github.com/zrygan/media-upload-service/consumer/queue"
	"github.com/zrygan/media-upload-service/consumer/server"
	"github.com/zrygan/media-upload-service/consumer/worker"
	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"github.com/zrygan/media-upload-service/util"
	"google.golang.org/grpc"
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

	_, err = server.CleanStaging(cc.StagingFolder)

	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", cc.ListenAddress)
	if err != nil {
		return err
	}

	q := queue.New(cc.QueueMaxSize)

	srv := grpc.NewServer()
	videouploadv1.RegisterVideoUploadServiceServer(srv, server.New(q, cc.StagingFolder))
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(lis)
	}()

	slog.Info("server started")

	cws := worker.MakeWorkers(cc.ThreadCount, q, cc.MediaFolder)

	done := make(chan struct{})
	go func() {
		worker.RunThreads(cws)
		close(done)
	}()

	slog.Info("consumer started", "queue_max_size", cc.QueueMaxSize, "thread_count", cc.ThreadCount, "listen_address", cc.ListenAddress, "media_folder", cc.MediaFolder, "staging_folder", cc.StagingFolder)

	var runErr error

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		slog.Error("server stopped unexpectedly", "error", err)
		runErr = err
	}

	srv.GracefulStop()
	q.Close()
	<-done
	slog.Info("server stopped")
	slog.Info("consumer stopped")

	return runErr
}
