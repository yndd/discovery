package discovery_rules

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/karimra/gnmic/target"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	discoveryv1alpha1 "github.com/yndd/discovery-operator/api/v1alpha1"
	"github.com/yndd/discovery-operator/discovery/discoverers"
	"github.com/yndd/ndd-runtime/pkg/logging"
	targetv1 "github.com/yndd/ndd-target-runtime/apis/dvr/v1"
	"github.com/yndd/ndd-target-runtime/pkg/ygotnddtarget"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
	namespace := dr.Spec.TargetNamespace
	if namespace == "" {
		namespace = dr.GetNamespace()
	}
	targetName := fmt.Sprintf("%s.%s.%s", *di.HostName, strings.Fields(*di.SerialNumber)[0], *di.MacAddress)
	targetName = strings.ReplaceAll(targetName, ":", "-")
	targetName = strings.ToLower(targetName)
	nddTarget := &ygotnddtarget.NddTarget_TargetEntry{
		AdminState: ygotnddtarget.NddCommon_AdminState_enable,
		Config: &ygotnddtarget.NddTarget_TargetEntry_Config{
			Address:        &t.Config.Address,
			CredentialName: pointer.String(dr.Spec.Credentials),
			Encoding:       ygotnddtarget.NddTarget_Encoding_ASCII,
			Insecure:       pointer.Bool(false),
			Protocol:       ygotnddtarget.NddTarget_Protocol_gnmi,
			// Proxy:          new(string),
			SkipVerify: pointer.Bool(true),
			// TlsCredentialName: new(string),
		},
		Description: pointer.String(fmt.Sprintf("discovered by rule %s", dr.GetName())),
		Name:        pointer.String(targetName),
		VendorType:  ygotnddtarget.NddTarget_VendorType_nokia_srl,
	}

	//
	j, err := ygot.EmitJSON(nddTarget, &ygot.EmitJSONConfig{
		Format:         ygot.RFC7951,
		SkipValidation: true,
	})
	if err != nil {
		return err
	}

	// check if the target already exists
	targetCR := &targetv1.Target{}
	err = c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      targetName,
	}, targetCR)
	if err != nil {
		if kerrors.IsNotFound(err) {
			targetCR = &targetv1.Target{
				ObjectMeta: metav1.ObjectMeta{
					Name:      targetName,
					Namespace: namespace,
					Labels:    map[string]string{},
					Annotations: map[string]string{
						"yndd.io/discovery-rule": dr.GetName(),
						"yndd.io/mgmt-address":   t.Config.Address,
					},
				},
				Spec: targetv1.TargetSpec{
					Properties: runtime.RawExtension{Raw: []byte(j)},
				},
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