// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Flag is a [flag.Value] that parses and handles a user provided trigger
// argument.
//
// Its common use case looks like this:
//
//	var trigger trigger.Flag
//	flag.Var(&trigger, "trigger", trigger.docs())
//	flag.Parse()
//
//	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer stop()
//
//	err := trigger.wait(ctx)
//	if err != nil {
//		// Handle error.
//	}
type Flag struct {
	// Either signal:SIGNAL or sleep:DURATION
	value string
	fn    func(context.Context) error
}

var _ flag.Value = (*Flag)(nil)

// Docs returns documentation about the Flag for the command line.
func (f *Flag) Docs() string {
	return `trigger to wait for (i.e. "signal:SIGCONT", "sleep:5s")`
}

// Set sets the trigger behavior of the Flag.
func (f *Flag) Set(s string) error {
	f.value = s

	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid trigger flag: %s", s)
	}

	var err error
	switch parts[0] {
	case "signal":
		f.fn, err = Signal(parts[1])
	case "sleep":
		f.fn, err = Sleep(parts[1])
	default:
		return fmt.Errorf("unknown trigger: %s", parts[0])
	}
	return err
}

// String returns the configured trigger value.
func (f *Flag) String() string {
	if f == nil {
		return ""
	}
	return f.value
}

// Wait will wait until ready or the passed context is done.
//
// A non-nil error is returned if one occurs during the wait. If the passed
// context is cancelled or times out, the context error is returned.
func (f *Flag) Wait(ctx context.Context) error {
	if f == nil || f.fn == nil {
		// Default to sleeping 5 seconds.
		time.Sleep(time.Second * 5)
		return nil
	}
	return f.fn(ctx)
}

// Sleep returns a trigger function that will wait for the parsed duration. An
// error is returned if the passed duration is invalid.
func Sleep(duration string) (func(context.Context) error, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("invalid sleep duration: %w", err)
	}
	return func(ctx context.Context) error {
		time.Sleep(d)
		return nil
	}, nil
}

// Signal returns a trigger function that will wait for the parsed signal s to
// be sent to the process. An error is returned if the passed signal is
// invalid.
func Signal(s string) (func(context.Context) error, error) {
	sig2 := unix.SignalNum(s)
	if sig2 == 0 {
		return nil, fmt.Errorf("invalid signal: %s", s)
	}
	return func(ctx context.Context) error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, sig2)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
		return nil
	}, nil
}
