package main

import (
	"os"
	"os/signal"
	"syscall"
	"sync/atomic"
)

var receivedSIGTERM = int32(0)

// RegisterHandler registers the handler for the SIGTERM signal.
func RegisterSIGTERMHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		atomic.AddInt32(&receivedSIGTERM, 1)
	}()
}

// Done returns true when a SIGTERM signal has been received.
func ReceivedSIGTERM() bool {
	return atomic.LoadInt32(&receivedSIGTERM) > 0
}
