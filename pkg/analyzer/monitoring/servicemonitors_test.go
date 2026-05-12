package monitoring

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestMapServiceMonitorsToServices(t *testing.T) {
	serviceMonitors := &monitoringv1.ServiceMonitorList{
		Items: []monitoringv1.ServiceMonitor{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "api-monitor"},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "api"},
					},
				},
			},
		},
	}
	services := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "api",
					Labels: map[string]string{"app": "api"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "web",
					Labels: map[string]string{"app": "web"},
				},
			},
		},
	}

	mappings := MapServiceMonitorsToServices(serviceMonitors, services)

	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
	if mappings[0].ServiceMonitorName != "api-monitor" || len(mappings[0].ServiceNames) != 1 || mappings[0].ServiceNames[0] != "api" {
		t.Fatalf("expected api-monitor to map to api service, got %#v", mappings[0])
	}
}

func TestCheckServiceMonitorPorts(t *testing.T) {
	serviceMonitors := &monitoringv1.ServiceMonitorList{
		Items: []monitoringv1.ServiceMonitor{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "api-monitor"},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "api"},
					},
					Endpoints: []monitoringv1.Endpoint{{Port: "metrics"}},
				},
			},
		},
	}
	services := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "api",
					Labels: map[string]string{"app": "api"},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
		},
	}

	statuses := CheckServiceMonitorPorts(serviceMonitors, services)

	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].ServiceMonitorName != "api-monitor" || len(statuses[0].RogueServices) != 1 || statuses[0].RogueServices[0] != "api" {
		t.Fatalf("expected api service to be marked rogue, got %#v", statuses[0])
	}
}
