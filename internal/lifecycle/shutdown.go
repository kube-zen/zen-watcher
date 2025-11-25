package lifecycle

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
		log.Printf("⚠️  Shutdown signal received: %v", sig)
		close(stopCh)
		cancel()
	}()

	return ctx, stopCh
}

// WaitForShutdown waits for all goroutines to finish and performs final cleanup
func WaitForShutdown(ctx context.Context, wg *sync.WaitGroup, cleanup func()) {
	<-ctx.Done()
	log.Println("⏳ Waiting for goroutines to finish...")
	wg.Wait()
	if cleanup != nil {
		cleanup()
	}
	log.Println("✅ Shutdown complete")
}
