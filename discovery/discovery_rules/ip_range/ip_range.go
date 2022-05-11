package ip_range

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/go-ping/ping"
	gapi "github.com/karimra/gnmic/api"
	discoveryv1alpha1 "github.com/yndd/discovery-operator/api/v1alpha1"
	discoveryrules "github.com/yndd/discovery-operator/discovery/discovery_rules"
	"github.com/yndd/ndd-runtime/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	hosts, err := getHosts(dr.Spec.IPranges...)
	if err != nil {
		return err
	}
	for _, e := range dr.Spec.Excludes {
		excludes, err := getHosts(e)
		if err != nil {
			return err
		}
		for h := range excludes {
			delete(hosts, h)
		}
	}
	//
	for _, ip := range sortIPs(hosts) {
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

func getHosts(cidrs ...string) (map[string]struct{}, error) {
	ips := make(map[string]struct{})
	for _, cidr := range cidrs {
		ip, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
			ips[ip.String()] = struct{}{}
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
	case "snmp":
		return nil
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
			gapi.Timeout(5 * time.Second),
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
		discoverer, err := discoveryrules.GetDiscovererGNMI(capRsp)
		if err != nil {
			return err
		}
		di, err := discoverer.Discover(ctx, dr, t)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(di)
		i.logger.Info("discovery info", "info", string(b))
		return discoveryrules.ApplyTarget(ctx, i.client, dr, di, t)
	}
}

func sortIPs(hosts map[string]struct{}) []string {
	realIPs := make([]net.IP, 0, len(hosts))

	for ip := range hosts {
		realIPs = append(realIPs, net.ParseIP(ip))
	}

	sort.Slice(realIPs, func(i, j int) bool {
		return bytes.Compare(realIPs[i], realIPs[j]) < 0
	})

	ips := make([]string, 0, len(realIPs))
	for _, rip := range realIPs {
		ips = append(ips, rip.String())
	}
	return ips
}
