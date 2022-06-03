package discovery_rules

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/karimra/gnmic/target"
	"github.com/openconfig/gnmi/proto/gnmi"
	discoveryv1alpha1 "github.com/yndd/discovery/api/v1alpha1"
	"github.com/yndd/discovery/internal/discovery/discoverers"
	"github.com/yndd/ndd-runtime/pkg/logging"
	targetv1 "github.com/yndd/target/apis/target/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IPRangeDiscoveryRule = "ipRange"
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

func GetDiscovererGNMI(capRsp *gnmi.CapabilityResponse) (discoverers.Discoverer, error) {
	var discoverer discoverers.Discoverer
OUTER:
	for _, m := range capRsp.SupportedModels {
		switch m.Organization {
		case "Nokia":
			if strings.Contains(m.Name, "srl_nokia") {
				// SRL
				init := discoverers.Discoverers[discoverers.NokiaSRLDiscovererName]
				discoverer = init()
			} else {
				// SROS
				init := discoverers.Discoverers[discoverers.NokiaSROSDiscovererName]
				discoverer = init()
			}
			break OUTER
		}
	}
	if discoverer == nil {
		return nil, errors.New("unknown target vendor")
	}
	return discoverer, nil
}

func ApplyTarget(ctx context.Context, c client.Client, dr *discoveryv1alpha1.DiscoveryRule, di *targetv1.DiscoveryInfo, t *target.Target) error {
	var namespace string
	if dr.Spec.TargetTemplate != nil {
		namespace = dr.Spec.TargetTemplate.Namespace
	}
	if namespace == "" {
		namespace = dr.GetNamespace()
	}
	targetName := fmt.Sprintf("%s.%s.%s", di.HostName, strings.Fields(di.SerialNumber)[0], di.MacAddress)
	targetName = strings.ReplaceAll(targetName, ":", "-")
	targetName = strings.ToLower(targetName)
	targetSpec := targetv1.TargetSpec{
		Properties: &targetv1.TargetProperties{
			VendorType: di.VendorType,
			Config: &targetv1.TargetConfig{
				Address:           t.Config.Address,
				CredentialName:    dr.Spec.Credentials,
				Encoding:          "",
				Insecure:          *t.Config.Insecure,
				Protocol:          targetv1.Protocol(targetv1.Protocol_GNMI),
				SkipVerify:        *t.Config.SkipVerify,
				TlsCredentialName: "",
			},
			// Allocation: map[string]*targetv1.Allocation{},
		},
	}

	// check if the target already exists
	targetCR := &targetv1.Target{}
	err := c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      targetName,
	}, targetCR)
	if err != nil {
		if kerrors.IsNotFound(err) {
			labels, err := dr.GetTargetLabels(&targetSpec)
			if err != nil {
				return err
			}
			anno, err := dr.GetTargetAnnotations(&targetSpec)
			if err != nil {
				return err
			}
			targetCR = &targetv1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:        targetName,
					Namespace:   namespace,
					Labels:      labels,
					Annotations: anno,
				},
				Spec: targetSpec,
			}
			err = c.Create(ctx, targetCR)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	// target already exists
	targetCR.Status = targetv1.TargetStatus{
		Status: targetv1.Status{
			DiscoveryInfo: di,
		},
	}
	return c.Status().Update(ctx, targetCR)
}

func Initialize(dr *discoveryv1alpha1.DiscoveryRule) DiscoveryRule {
	if dr.Spec.IPRange != nil {
		drInit, ok := DiscoveryRules[IPRangeDiscoveryRule]
		if !ok {
			return nil
		}
		return drInit()
	}
	return nil
}
