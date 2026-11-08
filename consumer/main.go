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

// run starts the gRPC server and the workers, then blocks until the
// consumer is stopped (Ctrl+C or SIGTERM) or the server fails. This
// returns nil on a normal shutdown; otherwise, it returns the error
// that stopped it.
//
// The shutdown happens in this order:
//
//  1. srv.GracefulStop stops new uploads and waits for the running
//     ones to finish, so nothing is added to the queue after this.
//  2. q.Close closes the queue. This MUST come after (1), since adding
//     to a closed queue panics.
//  3. <-done waits for the workers to empty the queue, so every
//     accepted upload is saved before the consumer exits.
func run() error {

	configPath := flag.String("config", ".configs/consumer.json", "path to the consumer config")
	flag.Parse()

	cc, err := config.ReadConfigFromJSON(*configPath)
	if err != nil {
		return err
	}

	// this must run before the server starts, see CleanStaging
	_, err = server.CleanStaging(cc.StagingFolder)

	if err != nil {
		return err
	}

	// ctx is cancelled on Ctrl+C or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", cc.ListenAddress)
	if err != nil {
		return err
	}

	q := queue.New(cc.QueueMaxSize)

	// Serve blocks, so it runs on its own goroutine
	srv := grpc.NewServer()
	videouploadv1.RegisterVideoUploadServiceServer(srv, server.New(q, cc.StagingFolder))
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(lis)
	}()

	slog.Info("server started")

	// done is closed once every worker has returned
	cws := worker.MakeWorkers(cc.ThreadCount, q, cc.MediaFolder)

	done := make(chan struct{})
	go func() {
		worker.RunThreads(cws)
		close(done)
	}()

	slog.Info("consumer started", "queue_max_size", cc.QueueMaxSize, "thread_count", cc.ThreadCount, "listen_address", cc.ListenAddress, "media_folder", cc.MediaFolder, "staging_folder", cc.StagingFolder)

	var runErr error

	// wait for a shutdown signal or for the server to fail
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
