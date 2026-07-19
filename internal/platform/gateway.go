package platform

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("local gateway discovery is not implemented for this platform")

type Gateway struct{ Address, Interface string }
type GatewayDiscoverer interface {
	Discover(context.Context) (Gateway, error)
}

type UnsupportedGatewayDiscoverer struct{}

func (UnsupportedGatewayDiscoverer) Discover(context.Context) (Gateway, error) {
	return Gateway{}, ErrUnsupported
}
