package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/kube-zen/zen-watcher/pkg/logger"
)

// SetupSignalHandler creates a context that cancels on SIGINT/SIGTERM
// Returns the context and a stop channel for informers
func SetupSignalHandler() (context.Context, chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	stopCh := make(chan struct{})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Shutdown signal received",
			logger.Fields{
				Component: "lifecycle",
				Operation: "signal_handler",
				Additional: map[string]interface{}{
					"signal": sig.String(),
				},
			})
		close(stopCh)
		cancel()
	}()

	return ctx, stopCh
}

// WaitForShutdown waits for all goroutines to finish and performs final cleanup
func WaitForShutdown(ctx context.Context, wg *sync.WaitGroup, cleanup func()) {
	<-ctx.Done()
	logger.Info("Waiting for goroutines to finish",
		logger.Fields{
			Component: "lifecycle",
			Operation: "shutdown_wait",
		})
	wg.Wait()
	if cleanup != nil {
		cleanup()
	}
	logger.Info("Shutdown complete",
		logger.Fields{
			Component: "lifecycle",
			Operation: "shutdown_complete",
		})
}
