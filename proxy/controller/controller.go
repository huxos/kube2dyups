package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/huxos/kube2dyups/proxy/template"
	khcache "github.com/huxos/kube2dyups/util/cache"

	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/golang/glog"
)

// ProxyController abstracts the details of watching Services and
// Endpoints from the Proxy implementation being used.
type ProxyController struct {
	lock sync.Mutex

	Proxier       *template.Proxier
	NextEndpoints func() (watch.EventType, *api.Endpoints, error)

	EndpointsListConsumed func() bool
	endpointsListConsumed bool
}

func New(client *kubeclient.Clientset, proxier *template.Proxier, resyncPeriod time.Duration, watchNamespace string) *ProxyController {
	endpointsEventQueue := khcache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(createEndpointsLW(client, watchNamespace), &api.Endpoints{}, endpointsEventQueue, resyncPeriod).Run()

	return &ProxyController{
		Proxier: proxier,
		NextEndpoints: func() (watch.EventType, *api.Endpoints, error) {
			eventType, obj, err := endpointsEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*api.Endpoints), nil
		},
		EndpointsListConsumed: func() bool {
			return endpointsEventQueue.ListConsumed()
		},
	}
}

// Run begins watching and syncing.
func (c *ProxyController) Run(stopCh <-chan struct{}) {
	glog.V(4).Infof("Running proxy controller")
	go wait.Until(c.HandleEndpoints, 0, stopCh)
	<-stopCh
}

// HandleEndpoints handles a single Endpoints event and refreshes the proxy backend.
func (c *ProxyController) HandleEndpoints() {
	eventType, endpoints, err := c.NextEndpoints()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to handle endpoints: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// Change the local sync state within the lock to ensure that all
	// event handlers have the same view of the sync state.
	c.endpointsListConsumed = c.EndpointsListConsumed()
	c.updateLastSyncProcessed()

	if err := c.Proxier.HandleEndpoints(eventType, endpoints); err != nil {
		utilruntime.HandleError(err)
	}
}

// updateLastSyncProcessed notifies the proxier if the most recent sync
// of resource has been completed.
func (c *ProxyController) updateLastSyncProcessed() {
	lastSyncProcessed := c.endpointsListConsumed
	c.Proxier.SetSkipCommit(!lastSyncProcessed)
}

func createEndpointsLW(client *kubeclient.Clientset, watchNamespace string) *cache.ListWatch {

	return cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "endpoints", watchNamespace, fields.Everything())
}
