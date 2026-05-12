package monitoring

import (
	"slices"

	"k8s-monitor-debugger/pkg/analyzer/labels"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
)

type ServiceMonitorServiceMapping struct {
	ServiceMonitorName         string
	ServiceNames               []string
	NoSelector                 bool
	UnsupportedMatchExpression bool
}

type ServiceMonitorPortStatus struct {
	ServiceMonitorName string
	RogueServices      []string
}

func MapServiceMonitorsToServices(serviceMonitors *monitoringv1.ServiceMonitorList, services *v1.ServiceList) []ServiceMonitorServiceMapping {
	mappings := []ServiceMonitorServiceMapping{}

	for _, serviceMonitor := range serviceMonitors.Items {
		mapping := ServiceMonitorServiceMapping{
			ServiceMonitorName: serviceMonitor.Name,
			NoSelector:         len(serviceMonitor.Spec.Selector.MatchLabels) == 0,
		}

		if mapping.NoSelector {
			mapping.UnsupportedMatchExpression = len(serviceMonitor.Spec.Selector.MatchExpressions) > 0
			mappings = append(mappings, mapping)
			continue
		}

		for _, service := range services.Items {
			if labels.Match(serviceMonitor.Spec.Selector.MatchLabels, service.Labels) {
				mapping.ServiceNames = append(mapping.ServiceNames, service.Name)
			}
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}

func CheckServiceMonitorPorts(serviceMonitors *monitoringv1.ServiceMonitorList, services *v1.ServiceList) []ServiceMonitorPortStatus {
	statuses := []ServiceMonitorPortStatus{}
	endpointsByServiceMonitor := endpointsForServiceMonitors(serviceMonitors)
	portsByService := portsForServices(services)
	serviceMappings := MapServiceMonitorsToServices(serviceMonitors, services)

	for _, mapping := range serviceMappings {
		if mapping.NoSelector || len(mapping.ServiceNames) == 0 {
			continue
		}

		status := ServiceMonitorPortStatus{
			ServiceMonitorName: mapping.ServiceMonitorName,
		}

		for _, serviceName := range mapping.ServiceNames {
			endpoints := endpointsByServiceMonitor[mapping.ServiceMonitorName]
			ports := portsByService[serviceName]
			if !sameIgnoringOrder(endpoints, ports) {
				status.RogueServices = append(status.RogueServices, serviceName)
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func portsForServices(services *v1.ServiceList) map[string][]string {
	portsByService := make(map[string][]string)
	for _, service := range services.Items {
		ports := []string{}
		for _, port := range service.Spec.Ports {
			ports = append(ports, port.Name)
		}
		portsByService[service.Name] = ports
	}
	return portsByService
}

func endpointsForServiceMonitors(serviceMonitors *monitoringv1.ServiceMonitorList) map[string][]string {
	endpointsByServiceMonitor := make(map[string][]string)
	for _, serviceMonitor := range serviceMonitors.Items {
		endpoints := []string{}
		for _, endpoint := range serviceMonitor.Spec.Endpoints {
			endpoints = append(endpoints, endpoint.Port)
		}
		endpointsByServiceMonitor[serviceMonitor.Name] = endpoints
	}
	return endpointsByServiceMonitor
}

func sameIgnoringOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := slices.Clone(a)
	bCopy := slices.Clone(b)

	slices.Sort(aCopy)
	slices.Sort(bCopy)

	return slices.Equal(aCopy, bCopy)
}
