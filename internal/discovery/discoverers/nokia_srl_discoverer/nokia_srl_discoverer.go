package nokia_srl_discoverer

import (
	"context"

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
	// TODO: check if we need to differentiate slotA and slotB
	srlSwVersionPath = "platform/control/software-version"
	srlChassisPath   = "platform/chassis"
	srlHostnamePath  = "system/name/host-name"
	//
	srlChassisTypePath  = "platform/chassis/type"
	srlSerialNumberPath = "platform/chassis/serial-number"
	srlHWMacAddrPath    = "platform/chassis/hw-mac-address"
)

func init() {
	discoverers.Register(discoverers.NokiaSRLDiscovererName, func() discoverers.Discoverer {
		return &srlDiscoverer{}
	})
}

type srlDiscoverer struct{}

func (s *srlDiscoverer) Discover(ctx context.Context, dr *discoveryv1alphav1.DiscoveryRule, t *target.Target) (*targetv1.DiscoveryInfo, error) {
	req, err := gapi.NewGetRequest(
		gapi.Path(srlSwVersionPath),
		gapi.Path(srlChassisPath),
		gapi.Path(srlHostnamePath),
		gapi.EncodingASCII(),
		gapi.DataTypeSTATE(),
	)
	if err != nil {
		return nil, err
	}
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	devDetails := &targetv1.DiscoveryInfo{
		VendorType: ygot.String(ygotnddtarget.NddTarget_VendorType_nokia_srl.String()),
	}
	for _, notif := range resp.GetNotification() {
		for _, upd := range notif.GetUpdate() {
			p := gutils.GnmiPathToXPath(upd.GetPath(), true)
			switch p {
			case srlSwVersionPath:
				if devDetails.SwVersion == nil {
					devDetails.SwVersion = utils.StringPtr(upd.GetVal().GetStringVal())
				}
			case srlChassisTypePath:
				devDetails.Kind = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlSerialNumberPath:
				devDetails.SerialNumber = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlHWMacAddrPath:
				devDetails.MacAddress = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlHostnamePath:
				devDetails.HostName = utils.StringPtr(upd.GetVal().GetStringVal())
			}
		}
	}
	return devDetails, nil
}
