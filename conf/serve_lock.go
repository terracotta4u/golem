package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// HoldServe takes an exclusive lock on ~/.golem/serve.lock until unlock.
// golem serve holds it for the process lifetime.
func HoldServe() (unlock func(), err error) {
	unlock, _, err = holdServe(false)
	return unlock, err
}

// TryHoldServe is the non-blocking form of HoldServe.
// ok is false when another process already holds the lock.
func TryHoldServe() (unlock func(), ok bool, err error) {
	return holdServe(true)
}

func holdServe(nonblock bool) (func(), bool, error) {
	dir, err := Dir()
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, fmt.Errorf("create %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "serve.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("lock serve: %w", err)
	}
	flags := unix.LOCK_EX
	if nonblock {
		flags |= unix.LOCK_NB
	}
	if err := unix.Flock(int(f.Fd()), flags); err != nil {
		f.Close()
		if nonblock && (errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lock serve: %w", err)
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, true, nil
}
