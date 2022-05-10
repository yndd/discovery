package discovery_rules

import (
	"context"

	discoveryv1alpha1 "github.com/yndd/discovery-operator/api/v1alpha1"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IPRangeDiscoveryRule = "ip-range"
)

type DiscoveryRule interface {
	Run(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, opts ...Option) error
	Stop() error
	//
	SetLogger(logger logging.Logger)
	SetClient(c client.Client)
}

type Initializer func() DiscoveryRule

var DiscoveryRules = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	DiscoveryRules[name] = initFn
}

type Option func(DiscoveryRule)

func WithLogger(logger logging.Logger) Option {
	return func(d DiscoveryRule) {
		d.SetLogger(logger)
	}
}

func WithClient(c client.Client) Option {
	return func(d DiscoveryRule) {
		d.SetClient(c)
	}
}
