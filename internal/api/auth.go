package api

import (
	"context"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Auth interface {
	New(context.Context, pp.Indent, time.Duration) (Handle, bool)
}
