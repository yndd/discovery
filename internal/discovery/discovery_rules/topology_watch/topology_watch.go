package topology_watch

import (
	"context"
	"encoding/json"
	"fmt"

	discoveryv1alpha1 "github.com/yndd/discovery/api/v1alpha1"
	discoveryrules "github.com/yndd/discovery/internal/discovery/discovery_rules"
	targetv1 "github.com/yndd/target/apis/target/v1"
	topologyv1alpha1 "github.com/yndd/topology/apis/topo/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/yndd/ndd-runtime/pkg/logging"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	discoveryrules.Register(discoveryrules.TopoWatchDiscoveryRule, func() discoveryrules.DiscoveryRule {
		return &topoWatch{}
	})
}

type topoWatch struct {
	logger logging.Logger
	client client.Client
	stopCh chan struct{}
}

func (i *topoWatch) Run(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, opts ...discoveryrules.Option) error {
	for _, o := range opts {
		o(i)
	}
	i.logger = i.logger.WithValues("discovery-rule", fmt.Sprintf("%s/%s", dr.GetNamespace(), dr.GetName()))

	return i.runNodeWatch(ctx, dr)
}

func (i *topoWatch) Stop() error {
	close(i.stopCh)
	return nil
}

func (i *topoWatch) SetLogger(logger logging.Logger) {
	i.logger = logger
}

func (i *topoWatch) SetClient(c client.Client) {
	i.client = c
}

//

func getNodeDynamicInformer(namespace string) (informers.GenericInformer, error) {
	gvrName := "nodes.v1alpha1.topo.yndd.io"
	cfg := ctrl.GetConfigOrDie()

	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, 0, namespace, nil)
	gvr, _ := schema.ParseResourceArg(gvrName)
	// create an informer
	informer := factory.ForResource(*gvr)
	return informer, nil
}

func (i *topoWatch) runNodeWatch(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule) error {
	nodeInformer, err := getNodeDynamicInformer(dr.Spec.TopologyRule.Namespace)
	if err != nil {
		return err
	}
	i.stopCh = make(chan struct{})
	i.runNodeInformer(ctx, dr, i.stopCh, nodeInformer.Informer())
	return nil
}

func (i *topoWatch) runNodeInformer(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, stopCh <-chan struct{}, s cache.SharedIndexInformer) {
	s.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    i.addNodeHandler(ctx, dr),
			DeleteFunc: i.deleteNodeHandler(ctx, dr),
			UpdateFunc: func(oldObj, newObj interface{}) {
				i.deleteNodeHandler(ctx, dr)(oldObj)
				i.addNodeHandler(ctx, dr)(newObj)
			},
		})
	s.Run(stopCh)
}

func (i *topoWatch) discover(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule, n *topologyv1alpha1.Node) error {
	switch dr.Spec.Protocol {
	case "snmp":
		return nil
	case "netconf":
		return nil
	default: // gnmi
		t, err := discoveryrules.CreateTarget(ctx, dr, i.client, n.Spec.Properties.MgmtIPAddress)
		if err != nil {
			return err
		}
		i.logger.Info("Creating gNMI client", "IP", t.Config.Name)
		err = t.CreateGNMIClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to create gNMI client: %w", err)
		}
		defer t.Close()
		capRsp, err := t.Capabilities(ctx)
		if err != nil {
			return fmt.Errorf("failed capabilities request: %w", err)
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
		return discoveryrules.ApplyTarget(ctx, i.client, dr, di, t, map[string]string{"topo.yndd.io/node": n.GetName()})
	}
}

func (i *topoWatch) addNodeHandler(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule) func(interface{}) {
	return func(obj interface{}) {
		n := &topologyv1alpha1.Node{}
		// https://erwinvaneyk.nl/kubernetes-unstructured-to-typed/
		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), n)
		if err != nil {
			i.logger.Info("convert failed", "error", err)
			return
		}
		i.logger.Info("node added", "node", n)
		err = i.discover(ctx, dr, n)
		if err != nil {
			i.logger.Info("node discovery failed", "node", n, "error", err)
			return
		}
		i.logger.Info("node discovered", "node", n)
	}
}

func (i *topoWatch) deleteNodeHandler(ctx context.Context, dr *discoveryv1alpha1.DiscoveryRule) func(interface{}) {
	return func(obj interface{}) {
		n := &topologyv1alpha1.Node{}
		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), n)
		if err != nil {
			i.logger.Info("convert failed", "error", err)
			return
		}
		i.logger.Info("node deleted", "node", n.GetName())
		tgList := &targetv1.TargetList{}
		validatedLabels, err := labels.ValidatedSelectorFromSet(map[string]string{
			"topo.yndd.io/node": n.GetName(),
		})
		if err != nil {
			i.logger.Info("failed to build label selector", "error", err)
			return
		}
		err = i.client.List(ctx, tgList, &client.ListOptions{
			LabelSelector: validatedLabels,
		})
		if err != nil {
			i.logger.Info("failed to list targets", "error", err)
			return
		}

		for _, tg := range tgList.Items {
			err = i.client.Delete(ctx, &tg)
			if err != nil {
				i.logger.Info("failed to delete target", "name", tg.GetName(), "error", err)
				continue
			}
			i.logger.Info("deleted target", "name", tg.GetName())
		}
	}
}
