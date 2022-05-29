package nokia_sros_discoverer

import (
	"context"
	"strings"

	gapi "github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/target"
	gutils "github.com/karimra/gnmic/utils"
	"github.com/openconfig/ygot/ygot"
	discoveryv1alphav1 "github.com/yndd/discovery/api/v1alpha1"
	"github.com/yndd/discovery/internal/discovery/discoverers"
	"github.com/yndd/ndd-runtime/pkg/utils"
	targetv1 "github.com/yndd/ndd-target-runtime/apis/dvr/v1"
	"github.com/yndd/ndd-target-runtime/pkg/ygotnddtarget"
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
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	devDetails := &targetv1.DiscoveryInfo{
		VendorType: ygot.String(ygotnddtarget.NddTarget_VendorType_nokia_sros.String()),
	}
	for _, notif := range resp.GetNotification() {
		for _, upd := range notif.GetUpdate() {
			p := gutils.GnmiPathToXPath(upd.GetPath(), true)
			switch p {
			case srosSWVersionPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				devDetails.SwVersion = utils.StringPtr(val)
			case srosChassisPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				devDetails.Kind = utils.StringPtr(val)
			case srosSerialNumberPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				devDetails.SerialNumber = utils.StringPtr(val)
			case srosHWMacAddressPath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				devDetails.MacAddress = utils.StringPtr(val)
			case srosHostnamePath:
				val := string(upd.GetVal().GetJsonVal())
				val = strings.Trim(val, "\"")
				devDetails.HostName = utils.StringPtr(val)
			}
		}
	}
	return devDetails, nil
}
