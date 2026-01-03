package progress

import (
	"fmt"
	"time"
)

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	// Match common CLI expectations: short tasks show sub-second precision.
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)

	switch {
	case n < kib:
		return fmt.Sprintf("%dB", n)
	case n < mib:
		return formatBytesUnit(float64(n)/kib, "KiB")
	case n < gib:
		return formatBytesUnit(float64(n)/mib, "MiB")
	case n < tib:
		return formatBytesUnit(float64(n)/gib, "GiB")
	default:
		return formatBytesUnit(float64(n)/tib, "TiB")
	}
}

func formatBytesUnit(v float64, unit string) string {
	// Keep output compact, while still being readable:
	// - < 10: 1 decimal
	// - otherwise: integer
	if v < 10 {
		return fmt.Sprintf("%.1f%s", v, unit)
	}
	return fmt.Sprintf("%.0f%s", v, unit)
}

func formatSpeed(bps float64) string {
	if bps <= 0 {
		return "?/s"
	}
	return fmt.Sprintf("%s/s", formatBytes(int64(bps)))
}
