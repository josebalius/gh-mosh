package mosh

import "context"

const moshVersion = "1.4.0"
const maxPacketSize = 1500
const moshKeyPrefix = "MOSH_KEY"

func await(ctx context.Context, errs chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
}
