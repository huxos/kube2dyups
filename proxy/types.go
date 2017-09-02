package proxy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
)
// ServiceUnit is an encapsulation of a service, the endpoints that back that service.
// This is the data that drives the creation of Nginx configuration file.
type ServiceUnit struct {
	// Name corresponds to ServicePortName. Uniquely identifies the ServiceUnit.
	Name string

	// Endpoints are endpoints that back the service, this translates into a final
	// backend implementation of Nginx.
	Endpoints []Endpoint
}

// ServicePortName is a combination of service.Namespace, service.Name and service.Ports[*].Name
type ServicePortName struct {
	types.NamespacedName
	Port string
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s_%s_%s", spn.NamespacedName.Namespace, spn.NamespacedName.Name, spn.Port)
}


// Endpoint is an internal representation of k8s Endpoint.
type Endpoint struct {
	IP   string
	Port int32
}

// Nginx upstream change Item
type UpsChangeItem struct {
	Name string
	Etype EventType
}