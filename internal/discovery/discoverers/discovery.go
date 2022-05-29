package discoverers

import (
	"context"

	"github.com/karimra/gnmic/target"
	discoveryv1alphav1 "github.com/yndd/discovery/api/v1alpha1"
	targetv1 "github.com/yndd/ndd-target-runtime/apis/dvr/v1"
)

const (
	NokiaSRLDiscovererName  = "nokia-srl"
	NokiaSROSDiscovererName = "nokia-sros"
)

// Discoverer discovers the target and returns discoveryInfo such as chassis type, SW version,
// SerialNumber, etc
type Discoverer interface {
	// Discover
	Discover(ctx context.Context, dr *discoveryv1alphav1.DiscoveryRule, t *target.Target) (*targetv1.DiscoveryInfo, error)
}

type Initializer func() Discoverer

var Discoverers = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Discoverers[name] = initFn
}
