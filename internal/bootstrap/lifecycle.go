package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Serve menjalankan server pada listener sampai ctx dibatalkan atau Serve gagal.
//
// Saat ctx dibatalkan, listener berhenti menerima koneksi baru dan request yang
// sedang berjalan diberi waktu grace untuk selesai. Jika grace habis, koneksi
// ditutup paksa (yang membatalkan context request). Serve selalu menunggu
// goroutine serve selesai sebelum kembali.
//
// Grace tidak bergantung pada ctx sehingga pembatalan sinyal tidak langsung
// membatalkan request aktif; jangan jadikan ctx sebagai BaseContext server.
func Serve(ctx context.Context, server *http.Server, listener net.Listener, grace time.Duration) error {
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	select {
	case err := <-serveDone:
		_ = listener.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("menjalankan server HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		// Grace habis: paksa tutup koneksi tersisa.
		if closeErr := server.Close(); closeErr != nil {
			shutdownErr = errors.Join(shutdownErr, closeErr)
		}
	}

	serveErr := <-serveDone
	_ = listener.Close()
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}

	var errs []error
	if serveErr != nil {
		errs = append(errs, fmt.Errorf("menjalankan server HTTP: %w", serveErr))
	}
	if shutdownErr != nil {
		errs = append(errs, fmt.Errorf("menghentikan server HTTP: %w", shutdownErr))
	}
	return errors.Join(errs...)
}
