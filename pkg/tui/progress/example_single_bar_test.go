package progress_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/tui/progress"
)

func ExampleSingleBar() {
	b := progress.NewSingleBar("Prefix")

	b.UpdateDisplay(&progress.DisplayProps{
		Prefix: "Prefix",
		Suffix: "Suffix",
	})

	n := 3

	go func() {
		time.Sleep(time.Second)
		for i := 0; i < n; i++ {
			b.UpdateDisplay(&progress.DisplayProps{
				Prefix: "Prefix" + strconv.Itoa(i),
				Suffix: "Suffix" + strconv.Itoa(i),
			})
			time.Sleep(time.Second)
		}
	}()

	b.StartRenderLoop()

	time.Sleep(time.Second * time.Duration(n+1))

	b.UpdateDisplay(&progress.DisplayProps{
		Mode: progress.ModeDone,
		// Mode:   progress.ModeError,
		Prefix: "Prefix",
	})

	b.StopRenderLoop()
}

func ExampleSingleBar_err() {
	b := progress.NewSingleBar("Prefix")

	b.UpdateDisplay(&progress.DisplayProps{
		Prefix: "Prefix",
		Suffix: "Suffix",
	})

	n := 3

	go func() {
		time.Sleep(time.Second)
		for i := 0; i < n; i++ {
			b.UpdateDisplay(&progress.DisplayProps{
				Prefix: "Prefix" + strconv.Itoa(i),
				Suffix: "Suffix" + strconv.Itoa(i),
			})
			time.Sleep(time.Second)
		}
	}()

	b.StartRenderLoop()

	time.Sleep(time.Second * time.Duration(n+1))

	b.UpdateDisplay(&progress.DisplayProps{
		Mode:   progress.ModeError,
		Prefix: "Prefix",
		Detail: errors.New("expected failure").Error(),
	})

	b.StopRenderLoop()
}

func TestExampleOutput(t *testing.T) {
	if !testing.Verbose() {
		return
	}
	ExampleSingleBar()
	ExampleSingleBar_err()
}
