package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/astromechza/todo-app/backend/api"
	"github.com/astromechza/todo-app/backend/model/sqlmodel"
)

func main() {
	if err := mainInner(); err != nil {
		slog.Error("exit with error", "err", err)
		os.Exit(1)
	}
}

//go:embed api.yaml
var ApiSpec []byte

func mainInner() error {

	connectCtx, connectCancel := context.WithTimeout(context.Background(), time.Second*10)
	defer connectCancel()

	db, err := sqlmodel.NewSqlModel(connectCtx, os.Getenv("DB_STRING"))
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close(connectCtx)

	apiServer := &api.Server{Database: db}

	echoServer := echo.New()
	echoServer.HidePort = true
	echoServer.HideBanner = true
	echoServer.HTTPErrorHandler = api.DefaultErrorHandler
	echoServer.JSONSerializer = new(api.DefaultJsonSerializer)
	if middleware, err := api.BuildOpenApiValidator(ApiSpec); err != nil {
		return err
	} else {
		echoServer.Use(middleware)
	}
	api.RegisterHandlers(echoServer, api.NewStrictHandler(apiServer, []api.StrictMiddlewareFunc{}))

	defer func() {
		if err := echoServer.Close(); err != nil {
			slog.Warn("closing server failed", "err", err)
		}
	}()

	listenError := make(chan error)
	go func() {
		addr := fmt.Sprintf(":%d", 8080)
		slog.Info("starting server", "addr", addr)
		if err := echoServer.Start(addr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				listenError <- err
			}
		}
	}()

	exit := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-exit:
		slog.Info("received signal", "signal", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := echoServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("stopping http server failed", "err", err)
		}
		return nil
	case err := <-listenError:
		return fmt.Errorf("failed to listen: %w", err)
	}
}
