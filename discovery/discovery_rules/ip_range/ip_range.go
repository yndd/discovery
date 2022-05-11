package ip_range

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ping/ping"
	gapi "github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/target"
	"github.com/openconfig/ygot/ygot"
	discoveryv1alpha1 "github.com/yndd/discovery-operator/api/v1alpha1"
	"github.com/yndd/discovery-operator/discovery/discoverers"
	discoveryrules "github.com/yndd/discovery-operator/discovery/discovery_rules"
	"github.com/yndd/ndd-runtime/pkg/logging"
	targetv1 "github.com/yndd/ndd-target-runtime/apis/dvr/v1"
	"github.com/yndd/ndd-target-runtime/pkg/ygotnddtarget"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	discoveryrules.Register(discoveryrules.IPRangeDiscoveryRule, func() discoveryrules.DiscoveryRule {
		return &ipRangeDR{}
	})
}

type ipRangeDR struct {
	client client.Client
	logger logging.Logger
	cfn    context.CancelFunc
}

func (i *ipRangeDR) Run(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, opts ...discoveryrules.Option) error {
	ctx, i.cfn = context.WithCancel(ctx)
	for _, o := range opts {
		o(i)
	}
	i.logger = i.logger.WithValues("discovery-rule", fmt.Sprintf("%s/%s", dr.GetNamespace(), dr.GetName()))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// run DR
			err := i.run(ctx, dr)
			if err != nil {
				i.logger.Info("failed to run discovery rule", "error", err)
			}
			time.Sleep(dr.Spec.Period.Duration)
		}
	}
}
func (i *ipRangeDR) Stop() error {
	i.cfn()
	return nil
}

//
func (i *ipRangeDR) SetLogger(logger logging.Logger) {
	i.logger = logger
}
func (i *ipRangeDR) SetClient(c client.Client) {
	i.client = c
}

//
func (i *ipRangeDR) run(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule) error {
	hosts, err := getHosts(dr.Spec.IPrange...)
	if err != nil {
		return err
	}
	ips := make([]string, 0, len(hosts))
	excludedHosts := make(map[string]struct{}, len(hosts))
	for _, e := range dr.Spec.Exclude {
		eh, err := getHosts(e)
		if err != nil {
			return err
		}
		for _, h := range eh {
			excludedHosts[h] = struct{}{}
		}
	}
	for _, h := range hosts {
		if _, ok := excludedHosts[h]; ok {
			continue
		}
		ips = append(ips, h)
	}
	//
	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err = pingIP(ip)
			if err != nil {
				i.logger.Info("Not reachable", "IP", ip, "error", err)
				continue
			}
			err := i.discover(ctx, dr, ip)
			if err != nil {
				i.logger.Info("Failed discovery", "IP", ip, "error", err)
				continue
			}
		}
	}
	return nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func getHosts(cidrs ...string) ([]string, error) {
	ips := make([]string, 0)
	for _, cidr := range cidrs {
		ip, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
			ips = append(ips, ip.String())
		}
	}
	return ips, nil
}

func pingIP(ip string) error {
	pinger := ping.New(ip)
	pinger.Count = 1
	pinger.Timeout = 1 * time.Second
	return pinger.Run()
}

func (i *ipRangeDR) discover(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, ip string) error {
	switch dr.Spec.Protocol {
	case "netconf":
		return nil
	default: // gnmi
		creds := &corev1.Secret{}
		err := i.client.Get(ctx, types.NamespacedName{
			Namespace: dr.GetNamespace(),
			Name:      dr.Spec.Credentials,
		}, creds)
		if err != nil {
			return err
		}
		tOpts := []gapi.TargetOption{
			gapi.Address(fmt.Sprintf("%s:%d", ip, dr.Spec.Port)),
			gapi.Username(string(creds.Data["username"])),
			gapi.Password(string(creds.Data["password"])),
		}
		if dr.Spec.Insecure {
			tOpts = append(tOpts, gapi.Insecure(true))
		} else {
			tOpts = append(tOpts, gapi.SkipVerify(true))
		}
		// TODO: query certificate, its secret and use it

		t, err := gapi.NewTarget(tOpts...)
		if err != nil {
			return err
		}
		defer t.Close()
		i.logger.Info("Creating gNMI client", "IP", t.Config.Name)
		err = t.CreateGNMIClient(ctx)
		if err != nil {
			return err
		}
		capRsp, err := t.Capabilities(ctx)
		if err != nil {
			return err
		}
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
			return errors.New("unknown target vendor")
		}
		di, err := discoverer.Discover(ctx, dr, t)
		if err != nil {
			return err
		}
		return i.applyTarget(ctx, dr, di, t)
	}
}

func (i *ipRangeDR) applyTarget(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, di *targetv1.DiscoveryInfo, t *target.Target) error {
	b, _ := json.Marshal(di)
	i.logger.Info("discovery info", "info", string(b))
	namespace := dr.Spec.TargetNamespace
	if namespace == "" {
		namespace = dr.GetNamespace()
	}
	targetName := fmt.Sprintf("%s.%s.%s.%s", dr.GetName(), *di.HostName, strings.Fields(*di.SerialNumber)[0], *di.MacAddress)
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
	err = i.client.Get(ctx, types.NamespacedName{
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
			err = i.client.Create(ctx, targetCR)
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
	return i.client.Status().Update(ctx, targetCR)
}
