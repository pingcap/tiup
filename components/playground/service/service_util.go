package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/cluster/api"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

func pollUntil(rt Runtime, interval, timeout time.Duration, probe func() (done bool, err error), onDone func()) {
	if rt == nil || probe == nil || onDone == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}

	start := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if rt.Stopping() {
			return
		}

		done, err := probe()
		if err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
		if done {
			onDone()
			return
		}
		if timeout > 0 && time.Since(start) >= timeout {
			fmt.Fprintln(rt.TermWriter(), "timeout waiting for scale-in")
			return
		}
		<-ticker.C
	}
}

func pdClient(rt Runtime) (*api.PDClient, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	if len(pds) == 0 {
		return nil, fmt.Errorf("no pd instance available")
	}
	addrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		addrs = append(addrs, pd.Addr())
	}
	ctx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, logprinter.NewLogger(""))
	return api.NewPDClient(ctx, addrs, 10*time.Second, nil), nil
}

func binlogClient(rt Runtime) (*api.BinlogClient, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	if len(pds) == 0 {
		return nil, fmt.Errorf("no pd instance available")
	}
	addrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		addrs = append(addrs, pd.Addr())
	}
	return api.NewBinlogClient(addrs, 5*time.Second, nil)
}

func dmMasterClient(rt Runtime) (*api.DMMasterClient, error) {
	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	if len(masters) == 0 {
		return nil, fmt.Errorf("no dm-master instance available")
	}
	addrs := make([]string, 0, len(masters))
	for _, master := range masters {
		addrs = append(addrs, master.Addr())
	}
	return api.NewDMMasterClient(addrs, 5*time.Second, nil), nil
}

func allocPort(host string, configured, defaultBase, portOffset int) int {
	base := configured
	if base <= 0 {
		base = defaultBase
	}
	return utils.MustGetFreePort(host, base, portOffset)
}
