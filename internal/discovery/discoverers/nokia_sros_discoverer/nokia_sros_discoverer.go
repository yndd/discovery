package nokia_sros_discoverer

import (
	"context"
	"strings"
	"time"

	gapi "github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/target"
	gutils "github.com/karimra/gnmic/utils"
	discoveryv1alphav1 "github.com/yndd/discovery/api/v1alpha1"
	"github.com/yndd/discovery/internal/discovery/discoverers"
	"github.com/yndd/ndd-runtime/pkg/utils"
	targetv1 "github.com/yndd/target/apis/target/v1"
)

const (
	srosSWVersionPath    = "state/system/version/version-number"
	srosChassisPath      = "state/system/platform"
	srosHostnamePath     = "state/system/oper-name"
	srosHWMacAddressPath = "state/system/base-mac-address"
	srosSerialNumberPath = "state/chassis/hardware-data/serial-number"
)

func init() {
	discoverers.Register(discoverers.NokiaSROSDiscovererName, func() discoverers.Discoverer {
		return &srosDiscoverer{}
	})
}

type srosDiscoverer struct{}

func (s *srosDiscoverer) Discover(ctx context.Context, dr *discoveryv1alphav1.DiscoveryRule, t *target.Target) (*targetv1.DiscoveryInfo, error) {
	req, err := gapi.NewGetRequest(
		gapi.Path(srosSWVersionPath),
		gapi.Path(srosChassisPath),
		gapi.Path(srosHWMacAddressPath),
		gapi.Path(srosHostnamePath),
		gapi.Path(srosSerialNumberPath),
		gapi.EncodingJSON(),
	)
	if err != nil {
		return nil, err
	}
	capRsp, err := t.Capabilities(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	di := &targetv1.DiscoveryInfo{
		VendorType:         targetv1.VendorTypeNokiaSROS,
		LastSeen:           time.Now().UnixNano(),
		SupportedEncodings: make([]string, 0, len(capRsp.GetSupportedEncodings())),
	}
	for _, enc := range capRsp.GetSupportedEncodings() {
		di.SupportedEncodings = append(di.SupportedEncodings, enc.String())
	}
	for _, notif := range resp.GetNotification() {
		for _, upd := range notif.GetUpdate() {
			p := gutils.GnmiPathToXPath(upd.GetPath(), true)
			switch p {
			case srosSWVersionPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				di.SwVersion = utils.StringPtr(val)
			case srosChassisPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				di.Platform = val
			case srosSerialNumberPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				di.SerialNumber = utils.StringPtr(val)
			case srosHWMacAddressPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				di.MacAddress = utils.StringPtr(val)
			case srosHostnamePath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				di.HostName = val
			}
		}
	}
	return di, nil
}
