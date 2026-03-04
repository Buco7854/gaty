package integration

import (
	"context"

	"github.com/Buco7854/gaty/internal/model"
)

// NoopDriver is a driver that does nothing. Used when an action is not configured.
type NoopDriver struct{}

func (d *NoopDriver) Execute(_ context.Context, _ *model.Gate) error { return nil }
