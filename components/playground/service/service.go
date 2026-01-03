package service

import (
	"fmt"
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
)

// Runtime is the minimal playground surface that service specs depend on.
//
// It is implemented by *main.Playground.
type Runtime interface {
	Booted() bool
	SharedOptions() proc.SharedOptions
	DataDir() string

	// BootConfig returns the boot-time base config for the given service.
	//
	// It is used to fill defaults for scale-out requests and to decide some
	// initial behaviors when creating instances.
	BootConfig(serviceID proc.ServiceID) (proc.Config, bool)

	Procs(serviceID proc.ServiceID) []proc.Process
	AddProc(serviceID proc.ServiceID, inst proc.Process)
	RemoveProc(serviceID proc.ServiceID, inst proc.Process) bool

	ExpectExitPID(pid int)
	Stopping() bool
	EmitEvent(evt any)

	TermWriter() io.Writer
	OnProcsChanged()
}

// Event is handled by playground's controller loop.
type Event interface {
	Handle(Runtime)
}

type NewProcParams struct {
	Config proc.Config
	ID     int
	Dir    string
	Host   string
}

type ScaleInHookFunc func(rt Runtime, w io.Writer, inst proc.Process, pid int) (async bool, err error)

type PostScaleOutFunc func(w io.Writer, inst proc.Process)

type Spec struct {
	ServiceID proc.ServiceID
	NewProc   func(rt Runtime, params NewProcParams) (proc.Process, error)

	// StartAfter declares boot-time ordering dependencies.
	//
	// Playground waits for all ready checks of the listed services (if any)
	// before starting this service.
	StartAfter []proc.ServiceID

	// ScaleInHook runs before the generic "expect-exit + SIGQUIT" path.
	//
	// If it returns async=true, the hook is responsible for arranging the actual
	// stop/removal (e.g. tombstone watchers), and the generic path is skipped.
	ScaleInHook ScaleInHookFunc

	// PostScaleOut is invoked after a scale-out instance is started successfully.
	PostScaleOut PostScaleOutFunc
}

// specs is intentionally treated as immutable after init() finishes.
// All registrations happen during package init, and runtime only performs reads.
var specs = make(map[proc.ServiceID]Spec)

func Register(spec Spec) error {
	if spec.ServiceID == "" {
		return fmt.Errorf("serviceID is empty")
	}
	if spec.NewProc == nil {
		return fmt.Errorf("service %s newProc is nil", spec.ServiceID)
	}
	if _, ok := specs[spec.ServiceID]; ok {
		return fmt.Errorf("duplicate service spec: %s", spec.ServiceID)
	}
	specs[spec.ServiceID] = spec
	return nil
}

func MustRegister(spec Spec) {
	if err := Register(spec); err != nil {
		panic(err.Error())
	}
}

func SpecFor(serviceID proc.ServiceID) (Spec, bool) {
	spec, ok := specs[serviceID]
	return spec, ok
}

func ProcsOf[T proc.Process](rt Runtime, serviceIDs ...proc.ServiceID) []T {
	if rt == nil || len(serviceIDs) == 0 {
		return nil
	}

	var out []T
	for _, serviceID := range serviceIDs {
		list := rt.Procs(serviceID)
		for _, inst := range list {
			v, ok := inst.(T)
			if !ok {
				panic(fmt.Sprintf("service %s has instance %T, want %T", serviceID, inst, *new(T)))
			}
			out = append(out, v)
		}
	}
	return out
}
