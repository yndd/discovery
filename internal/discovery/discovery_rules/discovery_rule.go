package discovery_rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gapi "github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/target"
	"github.com/openconfig/gnmi/proto/gnmi"
	discoveryv1alpha1 "github.com/yndd/discovery/api/v1alpha1"
	"github.com/yndd/discovery/internal/discovery/discoverers"
	"github.com/yndd/ndd-runtime/pkg/logging"
	targetv1 "github.com/yndd/target/apis/target/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IPRangeDiscoveryRule   = "ipRange"
	TopoWatchDiscoveryRule = "topoWatch"
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

func ApplyTarget(ctx context.Context,
	c client.Client, dr *discoveryv1alpha1.DiscoveryRule,
	di *targetv1.DiscoveryInfo, t *target.Target,
	drLabels map[string]string,
) error {
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
		DiscoveryInfo: di,
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
			// merge discovery rule implementation labels
			for k, v := range drLabels {
				labels[k] = v
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
	targetCR.Spec.DiscoveryInfo = di
	return c.Update(ctx, targetCR)
}

func Initialize(dr *discoveryv1alpha1.DiscoveryRule) DiscoveryRule {
	var ruleName string
	switch {
	case dr.Spec.IPRange != nil:
		ruleName = IPRangeDiscoveryRule
	case dr.Spec.TopologyRule != nil:
		ruleName = TopoWatchDiscoveryRule
	}
	drInit, ok := DiscoveryRules[ruleName]
	if !ok {
		return nil
	}
	return drInit()
}
func CreateTarget(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, c client.Client, ip string) (*target.Target, error) {
	creds := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{
		Namespace: dr.GetNamespace(),
		Name:      dr.Spec.Credentials,
	}, creds)
	if err != nil {
		return nil, err
	}
	tOpts := []gapi.TargetOption{
		gapi.Address(fmt.Sprintf("%s:%d", ip, dr.Spec.Port)),
		gapi.Username(string(creds.Data["username"])),
		gapi.Password(string(creds.Data["password"])),
		gapi.Timeout(5 * time.Second),
	}
	if dr.Spec.Insecure {
		tOpts = append(tOpts, gapi.Insecure(true))
	} else {
		tOpts = append(tOpts, gapi.SkipVerify(true))
	}
	// TODO: query certificate, its secret and use it

	return gapi.NewTarget(tOpts...)
}
