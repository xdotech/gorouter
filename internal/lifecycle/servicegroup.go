// Package lifecycle provides service lifecycle management.
// Services are started concurrently and stopped in reverse order (LIFO).
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Service is the interface each managed component must implement.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ServiceGroup manages a collection of services with coordinated lifecycle.
type ServiceGroup struct {
	services []Service
	stopOnce sync.Once
}

// NewServiceGroup creates an empty ServiceGroup.
func NewServiceGroup() *ServiceGroup {
	return &ServiceGroup{}
}

// Add registers a service to the group.
func (sg *ServiceGroup) Add(svc Service) {
	sg.services = append(sg.services, svc)
}

// Start launches all services concurrently and blocks until a termination
// signal (SIGINT/SIGTERM) is received or any service returns an error.
func (sg *ServiceGroup) Start(ctx context.Context) error {
	if len(sg.services) == 0 {
		return fmt.Errorf("no services to start")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(sg.services))

	for _, svc := range sg.services {
		svc := svc
		go func() {
			slog.Info("starting service", "name", svc.Name())
			if err := svc.Start(ctx); err != nil {
				slog.Error("service failed", "name", svc.Name(), "error", err)
				errCh <- fmt.Errorf("service %s: %w", svc.Name(), err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var exitErr error
	select {
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		exitErr = err
	case <-ctx.Done():
		exitErr = ctx.Err()
	}

	cancel()

	if err := sg.Stop(context.Background()); err != nil {
		slog.Error("error during shutdown", "error", err)
		if exitErr == nil {
			exitErr = err
		}
	}

	return exitErr
}

// Stop shuts down all services in reverse order (LIFO).
// Safe to call multiple times.
func (sg *ServiceGroup) Stop(ctx context.Context) error {
	var stopErr error
	sg.stopOnce.Do(func() {
		slog.Info("shutting down services", "count", len(sg.services))
		for i := len(sg.services) - 1; i >= 0; i-- {
			svc := sg.services[i]
			slog.Info("stopping service", "name", svc.Name())
			if err := svc.Stop(ctx); err != nil {
				slog.Error("service stop error", "name", svc.Name(), "error", err)
				if stopErr == nil {
					stopErr = fmt.Errorf("service %s stop: %w", svc.Name(), err)
				}
			}
		}
		slog.Info("all services stopped")
	})
	return stopErr
}

// FuncService wraps start/stop function pairs as a Service.
func FuncService(name string, start func(ctx context.Context) error, stop func(ctx context.Context) error) Service {
	return &funcService{name: name, start: start, stop: stop}
}

type funcService struct {
	name  string
	start func(ctx context.Context) error
	stop  func(ctx context.Context) error
}

func (f *funcService) Name() string                    { return f.name }
func (f *funcService) Start(ctx context.Context) error { return f.start(ctx) }
func (f *funcService) Stop(ctx context.Context) error  { return f.stop(ctx) }
