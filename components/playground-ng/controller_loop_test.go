package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestControllerLoop_DrainsEventsAfterCancel(t *testing.T) {
	p := NewPlayground("", 0)
	p.cmdReqCh = make(chan commandRequest)
	p.evtCh = make(chan controllerEvent, 1)
	p.controllerDoneCh = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p.evtCh <- stopInternalEvent{}
	go p.controllerLoop(ctx)

	select {
	case <-p.stoppingCh:
	case <-time.After(time.Second):
		require.FailNow(t, "stop event not handled after controller cancel")
	}

	select {
	case <-p.controllerDoneCh:
	case <-time.After(time.Second):
		require.FailNow(t, "controller did not exit after draining events")
	}
}
