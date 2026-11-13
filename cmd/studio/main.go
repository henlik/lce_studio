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

	"software-studio/internal/server"
	"software-studio/internal/store"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer db.Close()
	addr := env("STUDIO_ADDR", "127.0.0.1:8080")
	h := server.New(db, env("STUDIO_WEB_DIR", "web/dist"))
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	serverError := make(chan error, 1)
	go func() {
		serverError <- srv.ListenAndServe()
	}()
	log.Printf("LCE Studio v0.8.1 listening at %s (local single-user mode)", server.AddrForLog(addr))
	select {
	case <-stop.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
			_ = srv.Close()
		}
	case err := <-serverError:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server: %w", err)
		}
	}
	return nil
}
