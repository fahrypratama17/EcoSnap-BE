package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/bootstrap"
	"github.com/fahrypratama17/EcoSnap-BE/internal/config"
	firebaseintegration "github.com/fahrypratama17/EcoSnap-BE/internal/integration/firebase"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("membaca konfigurasi: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		pool.Close()
		log.Println("Pool PostgreSQL ditutup")
	}()

	log.Println("Koneksi PostgreSQL berhasil")

	authClient, err := firebaseintegration.NewAuthClient(ctx)
	if err != nil {
		return err
	}

	server := bootstrap.NewServer(cfg, authClient, pool)

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("membuka listener HTTP: %w", err)
	}

	log.Printf("EcoSnap HTTP server listening on %s", listener.Addr())

	if err := bootstrap.Serve(ctx, server, listener, 10*time.Second); err != nil {
		return err
	}

	if ctx.Err() != nil {
		stop()
		log.Println("Sinyal berhenti diterima, server HTTP berhenti dengan rapi")
	}
	return nil
}
