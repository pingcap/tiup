package utils

// Retry when the when func returns true
func Retry(f func() error, when func(error) bool) error {
	e := f()
	if e == nil {
		return nil
	}
	if when == nil {
		return Retry(f, nil)
	} else if when(e) {
		return Retry(f, when)
	}
	return nil
}
