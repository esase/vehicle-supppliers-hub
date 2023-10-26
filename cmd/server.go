//go:build !integration

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"bitbucket.org/crgw/service-helpers/logger"
	"bitbucket.org/crgw/supplier-hub/internal/tools/redisfactory"
	"bitbucket.org/crgw/supplier-hub/internal/web"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func serverApp(httpServer *http.Server, logger *zerolog.Logger) int {
	shutdown := false
	done := make(chan error, 1)
	stop := make(chan os.Signal)
	go func() {
		logger.
			Info().
			Msg("Listening on address " + httpServer.Addr)
		done <- httpServer.ListenAndServe()
	}()
	go func() {
		// Wait for stop
		<-stop
		shutdown = true
		logger.Info().Msg("Shutting down server...")
		_ = httpServer.Shutdown(context.Background())
	}()

	// Notify stop channel if SIGINT or SIGTERM is received
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	err := <-done
	if err != nil && !shutdown {
		logger.
			Error().
			Err(err).
			Msg("Server failed")
		return 1
	}
	return 0
}

func main() {
	_ = godotenv.Load(".env")
	log := logger.New(os.Getenv("LOG_LEVEL"))

	redisFactory := redisfactory.New()

	appRouter := web.SetupRouter(log, redisFactory)

	var host string
	if os.Getenv("TEST") == "true" {
		host = "localhost"
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, os.Getenv("PORT")),
		Handler: appRouter,
	}

	os.Exit(serverApp(httpServer, log))
}
