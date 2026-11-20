package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	log.Printf("EcoSnap HTTP server listening on %s", cfg.HTTPAddr)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("menjalankan server HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		stop()
		log.Println("Sinyal berhenti diterima, menghentikan server...")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("menghentikan server HTTP: %w", err)
	}

	log.Println("Server HTTP berhenti dengan rapi")
	return nil
}
