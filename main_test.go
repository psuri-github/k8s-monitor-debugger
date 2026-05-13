package main

import (
	"testing"

	monitoringanalyzer "k8s-monitor-debugger/pkg/analyzer/monitoring"
	"k8s-monitor-debugger/pkg/analyzer/workloads"

	v1 "k8s.io/api/core/v1"
)

func TestExitCodeForReport(t *testing.T) {
	tests := []struct {
		name     string
		report   report
		expected int
	}{
		{
			name: "clean report exits zero",
			report: report{
				services: &v1.ServiceList{},
				pods:     &v1.PodList{},
			},
			expected: exitOK,
		},
		{
			name: "missing service monitor crd exits one",
			report: report{
				services:              &v1.ServiceList{},
				pods:                  &v1.PodList{},
				serviceMonitorSkipped: true,
			},
			expected: exitIssuesFound,
		},
		{
			name: "service without pods exits one",
			report: report{
				services: &v1.ServiceList{},
				pods:     &v1.PodList{},
				servicePodMappings: []workloads.ServicePodMapping{
					{ServiceName: "api"},
				},
			},
			expected: exitIssuesFound,
		},
		{
			name: "service monitor without services exits one",
			report: report{
				services: &v1.ServiceList{},
				pods:     &v1.PodList{},
				serviceMonitorMappings: []monitoringanalyzer.ServiceMonitorServiceMapping{
					{ServiceMonitorName: "api-monitor"},
				},
			},
			expected: exitIssuesFound,
		},
		{
			name: "port mismatch exits one",
			report: report{
				services: &v1.ServiceList{},
				pods:     &v1.PodList{},
				serviceMonitorPorts: []monitoringanalyzer.ServiceMonitorPortStatus{
					{ServiceMonitorName: "api-monitor", RogueServices: []string{"api"}},
				},
			},
			expected: exitIssuesFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := exitCodeForReport(test.report)
			if actual != test.expected {
				t.Fatalf("expected exit code %d, got %d", test.expected, actual)
			}
		})
	}
}
