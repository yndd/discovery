package nokia_srl_discoverer

import (
	"context"
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
	capRsp, err := t.Capabilities(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	di := &targetv1.DiscoveryInfo{
		VendorType:         targetv1.VendorTypeNokiaSRL,
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
			case srlSwVersionPath:
				di.SwVersion = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlChassisTypePath:
				di.Platform = upd.GetVal().GetStringVal()
			case srlSerialNumberPath:
				di.SerialNumber = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlHWMacAddrPath:
				di.MacAddress = utils.StringPtr(upd.GetVal().GetStringVal())
			case srlHostnamePath:
				di.HostName = upd.GetVal().GetStringVal()
			}
		}
	}
	return di, nil
}
