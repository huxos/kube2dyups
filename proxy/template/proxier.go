package template

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"github.com/huxos/kube2dyups/proxy"
	utilnginx "github.com/huxos/kube2dyups/util/nginx"
	"github.com/huxos/kube2dyups/util/ratelimiter"
	"github.com/huxos/kube2dyups/util/template"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"github.com/golang/glog"
)

type ProxierConfig struct {
	NginxConfig utilnginx.NginxConfig
}

type Proxier struct {
	config ProxierConfig

	// if last sync has not processed, we should skip commit
	skipCommit bool

	// lock is a mutex used to protect below fields
	lock          sync.Mutex
	nginxInstance *utilnginx.Nginx

	routeTable map[string]*proxy.ServiceUnit

	rateLimitedCommitFuncForNginx *ratelimiter.RateLimitedFunction
	// rateLimitedCommitStopChannel is the stop/terminate channel.
	rateLimitedCommitStopChannel chan struct{}
}

func NewProxier(cfg ProxierConfig) (*Proxier, error) {
	proxier := &Proxier{
		config:                        cfg,
		rateLimitedCommitFuncForNginx: nil,
		rateLimitedCommitStopChannel:  make(chan struct{}),
		routeTable:                    make(map[string]*proxy.ServiceUnit),
	}

	nginx, err := utilnginx.NewInstance(cfg.NginxConfig)
	if err != nil {
		return nil, err
	}
	proxier.nginxInstance = nginx

	proxier.enableNginxRateLimiter(cfg.NginxConfig.DumpInterval, proxier.commitAndDumpConfig)

	return proxier, nil
}

func (proxier *Proxier) enableNginxRateLimiter(interval time.Duration, handlerFunc ratelimiter.HandlerFunc) {
	proxier.rateLimitedCommitFuncForNginx = ratelimiter.NewRateLimitedFunction("proxier_nginx", interval, handlerFunc)
	proxier.rateLimitedCommitFuncForNginx.RunUntil(proxier.rateLimitedCommitStopChannel)
}

func (proxier *Proxier) handleEndpointsAdd(endpoints *api.Endpoints) {
	proxier.lock.Lock()
	defer proxier.lock.Unlock()

	epName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}
	glog.V(4).Infof("handle endpoints %s add event", epName.String())

	if len(endpoints.Subsets) == 0 {
		// Skip endpoints as endpoints has no subsets
		glog.V(4).Infof("Skipping endpoints %s due to empty subsets", epName.String())
		return
	}

	changeList := []proxy.UpsChangeItem{}
	// We need to build a map of portname -> all ip:ports for that
	// portname. Explode Endpoints.Subsets[*] into this structure.
	portsToEndpoints := map[string][]proxy.Endpoint{}
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		if len(ss.NotReadyAddresses) != 0 {
			glog.V(4).Infof("endpoints %s has not ready address, please check this", epName.String())
		}
		for i := range ss.Ports {
			port := &ss.Ports[i]
			for i := range ss.Addresses {
				addr := &ss.Addresses[i]
				portsToEndpoints[port.Name] = append(portsToEndpoints[port.Name], proxy.Endpoint{addr.IP, port.Port})
			}
		}
	}

	for portname := range portsToEndpoints {
		svcPortName := proxy.ServicePortName{NamespacedName: epName, Port: portname}
		newEndpoints := portsToEndpoints[portname]
		if oldServiceUnit, exist := proxier.findServiceUnit(svcPortName.String()); exist {
			if !reflect.DeepEqual(oldServiceUnit.Endpoints, newEndpoints) {
				glog.V(4).Infof("endpoints of %s changed, needs update %v", svcPortName.String(), newEndpoints)
				oldServiceUnit.Endpoints = newEndpoints
				changeList = append(changeList, proxy.UpsChangeItem{
					Name:  svcPortName.String(),
					Etype: proxy.Modified,
				})
			}
		} else {
			glog.V(4).Infof("endpoints of %s not exist, needs add %v", svcPortName.String(), newEndpoints)
			newServiceUnit := proxier.createServiceUnit(svcPortName.String())
			newServiceUnit.Endpoints = append(newServiceUnit.Endpoints, newEndpoints...)
			changeList = append(changeList, proxy.UpsChangeItem{
				Name:  svcPortName.String(),
				Etype: proxy.Added,
			})
		}
	}
	proxier.commitNginx(changeList)
}

func (proxier *Proxier) handleEndpointsUpdate(endpoints *api.Endpoints) {
	epName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}
	proxier.lock.Lock()
	defer proxier.lock.Unlock()

	glog.V(4).Infof("handle endpoints %s update event", epName.String())

	if len(endpoints.Subsets) == 0 {
		glog.V(4).Infof("endpoints %s has empty subsets, this may happen when deploy", epName.String())
	}

	changeList := []proxy.UpsChangeItem{}
	oldSvcPortsList := proxier.getServicePorts(fmt.Sprintf("%s_%s", epName.Namespace, epName.Name))
	newSvcPortsList := []string{}

	// We need to build a map of portname -> all ip:ports for that
	// portname. Explode Endpoints.Subsets[*] into this structure.
	portsToEndpoints := map[string][]proxy.Endpoint{}
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		if len(ss.NotReadyAddresses) != 0 {
			glog.V(4).Infof("endpoints %s has not ready address, please check this", epName.String())
		}
		for i := range ss.Ports {
			port := &ss.Ports[i]
			for i := range ss.Addresses {
				addr := &ss.Addresses[i]
				portsToEndpoints[port.Name] = append(portsToEndpoints[port.Name], proxy.Endpoint{addr.IP, port.Port})
			}
		}
	}

	for portname := range portsToEndpoints {
		svcPortName := proxy.ServicePortName{NamespacedName: epName, Port: portname}
		newEndpoints := portsToEndpoints[portname]
		if oldServiceUnit, exist := proxier.findServiceUnit(svcPortName.String()); exist {
			if !reflect.DeepEqual(oldServiceUnit.Endpoints, newEndpoints) {
				glog.V(4).Infof("endpoints of %s changed, needs update %v", svcPortName.String(), newEndpoints)
				oldServiceUnit.Endpoints = newEndpoints
				changeList = append(changeList, proxy.UpsChangeItem{
					Name:  svcPortName.String(),
					Etype: proxy.Modified,
				})
			}
		} else {
			glog.V(4).Infof("endpoints of %s not exist, needs add %v", svcPortName.String(), newEndpoints)
			newServiceUnit := proxier.createServiceUnit(svcPortName.String())
			newServiceUnit.Endpoints = append(newServiceUnit.Endpoints, newEndpoints...)
			changeList = append(changeList, proxy.UpsChangeItem{
				Name:  svcPortName.String(),
				Etype: proxy.Added,
			})
		}
		newSvcPortsList = append(newSvcPortsList, svcPortName.String())
	}

	diffList := diff(oldSvcPortsList, newSvcPortsList)
	for _, diff := range diffList {
		//proxier.routeTable[diff].Endpoints = []proxy.Endpoint{}
		delete(proxier.routeTable, diff)
		changeList = append(changeList, proxy.UpsChangeItem{
			Name:  diff,
			Etype: proxy.Deleted,
		})
	}
	proxier.commitNginx(changeList)
}

func (proxier *Proxier) handleEndpointsDelete(endpoints *api.Endpoints) {
	proxier.lock.Lock()
	defer proxier.lock.Unlock()

	changeList := []proxy.UpsChangeItem{}
	epName := types.NamespacedName{
		Namespace: endpoints.Namespace,
		Name:      endpoints.Name,
	}
	glog.V(4).Infof("handle endpoints %s delete event", epName.String())

	portsNameMap := map[string]bool{}
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		for i := range ss.Ports {
			port := &ss.Ports[i]
			portsNameMap[port.Name] = true
		}
	}

	for portname := range portsNameMap {
		svcPort := proxy.ServicePortName{NamespacedName: types.NamespacedName{Namespace: endpoints.Namespace, Name: endpoints.Name}, Port: portname}
		if _, ok := proxier.findServiceUnit(svcPort.String()); ok {
			delete(proxier.routeTable, svcPort.String())
			changeList = append(changeList, proxy.UpsChangeItem{
				Name:  svcPort.String(),
				Etype: proxy.Deleted,
			})
		}
	}
	proxier.commitNginx(changeList)
}

func (proxier *Proxier) commitNginx(changeList []proxy.UpsChangeItem) error {
	if len(changeList) == 0 {
		glog.V(4).Infof("endpoint update event, but no backend changed")
	} else {
		glog.V(4).Infof("commit nginx")
		if proxier.skipCommit {
			glog.V(4).Infof("skipping nginx config dump for state: SkipCommit(%t)", proxier.skipCommit)
		} else {
			proxier.rateLimitedCommitFuncForNginx.Invoke(proxier.rateLimitedCommitFuncForNginx)
		}
		glog.V(4).Infof("%d ups updated: %v", len(changeList), changeList)
		for _, ups := range changeList {
			if ups.Etype == proxy.Added || ups.Etype == proxy.Modified {
				cfgBytes, err := template.RenderUpstreamTemplate("upstream", proxier.config.NginxConfig.TemplateDir, proxier.routeTable[ups.Name])
				if err != nil {
					return err
				}
				glog.V(4).Infof("post nginx upstream %s config:\n%s", ups.Name, string(cfgBytes))
				if err := proxier.nginxInstance.DyupsAddOrUpdate(ups.Name, cfgBytes); err != nil {
					return err
				}
			} else if ups.Etype == proxy.Deleted {
				glog.V(4).Infof("delete upstream %s", ups.Name)
				err := proxier.nginxInstance.DyupsDel(ups.Name)
				return err
			}
		}
	}
	return nil
}

func (proxier *Proxier) commitAndDumpConfig() error {
	proxier.lock.Lock()
	templateData := template.TemplateData{RouteTable: proxier.routeTable}
	cfgBytes, err := template.RenderUpstreamsTemplate("nginx_tpl", proxier.config.NginxConfig.TemplateDir, templateData)
	if err != nil {
		proxier.lock.Unlock()
		return err
	}
	proxier.lock.Unlock()

	// Dump Nginx Config Nginx
	if err := proxier.nginxInstance.DumpConfig(cfgBytes); err != nil {
		return err
	}
	return nil
}

// createServiceUnit creates a new service unit with given name.
func (proxier *Proxier) createServiceUnit(name string) *proxy.ServiceUnit {
	service := &proxy.ServiceUnit{
		Name:      name,
		Endpoints: []proxy.Endpoint{},
	}

	proxier.routeTable[name] = service
	return service
}

// getServicePorts returns a service ports name list.
func (proxier *Proxier) getServicePorts(svcName string) []string {
	names := []string{}
	for name, _ := range proxier.routeTable {
		svcNameExist := name[:strings.LastIndex(name, "_")]
		if svcNameExist == svcName {
			names = append(names, name)
		}
	}
	return names
}

// findServiceUnit finds the service unit with given name.
func (proxier *Proxier) findServiceUnit(name string) (*proxy.ServiceUnit, bool) {
	v, ok := proxier.routeTable[name]
	return v, ok
}

func (proxier *Proxier) HandleEndpoints(eventType watch.EventType, endpoints *api.Endpoints) error {
	switch eventType {
	case watch.Added:
		proxier.handleEndpointsAdd(endpoints)
	case watch.Modified:
		proxier.handleEndpointsUpdate(endpoints)
	case watch.Deleted:
		proxier.handleEndpointsDelete(endpoints)
	}

	return nil
}

// SetSkipCommit indicates to the proxier whether requests to
// commit/reload should be skipped.
func (proxier *Proxier) SetSkipCommit(skipCommit bool) {
	if proxier.skipCommit != skipCommit {
		glog.V(4).Infof("Updating skipCommit to %t", skipCommit)
		proxier.skipCommit = skipCommit
	}
}

// diff will calculate the diff between the old&new list.
func diff(oldList, newList []string) []string {
	result := []string{}
	for _, oldItem := range oldList {
		found := false
		for _, newItem := range newList {
			if oldItem == newItem {
				found = true
				break
			}
		}
		if !found {
			result = append(result, oldItem)
		}
	}
	return result
}
