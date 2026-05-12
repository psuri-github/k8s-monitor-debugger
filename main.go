package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	monitoringanalyzer "k8s-monitor-debugger/pkg/analyzer/monitoring"
	"k8s-monitor-debugger/pkg/analyzer/workloads"
	"k8s-monitor-debugger/pkg/kube"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type report struct {
	clusterEndpoint        string
	namespace              string
	services               *v1.ServiceList
	pods                   *v1.PodList
	serviceMonitors        *monitoringv1.ServiceMonitorList
	serviceMonitorCount    int
	servicePodMappings     []workloads.ServicePodMapping
	serviceMonitorMappings []monitoringanalyzer.ServiceMonitorServiceMapping
	serviceMonitorPorts    []monitoringanalyzer.ServiceMonitorPortStatus
}

func main() {
	namespace := flag.String("namespace", "monitoring", "Kubernetes namespace to inspect")
	verbose := flag.Bool("verbose", false, "Print detailed Services, Pods, and ServiceMonitors")
	flag.Parse()

	if *namespace == "" {
		log.Fatal("namespace cannot be empty")
	}

	config, err := kube.GetConfig()
	if err != nil {
		log.Fatalf("failed to create kube config: %v", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create kube client: %v", err)
	}

	monitorclient, err := monitoringclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create monitoring client: %v", err)
	}

	report, err := buildReport(context.Background(), config.Host, *namespace, client, monitorclient)
	if err != nil {
		log.Fatal(err)
	}

	printReport(report, *verbose)
}

func buildReport(ctx context.Context, clusterEndpoint string, namespace string, client *kubernetes.Clientset, monitorclient *monitoringclient.Clientset) (report, error) {
	services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return report{}, fmt.Errorf("failed to list services in namespace %q: %w", namespace, err)
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return report{}, fmt.Errorf("failed to list pods in namespace %q: %w", namespace, err)
	}

	serviceMonitors, err := monitorclient.MonitoringV1().ServiceMonitors(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return report{}, fmt.Errorf("failed to list serviceMonitors in namespace %q: %w", namespace, err)
	}

	return report{
		clusterEndpoint:        clusterEndpoint,
		namespace:              namespace,
		services:               services,
		pods:                   pods,
		serviceMonitors:        serviceMonitors,
		serviceMonitorCount:    len(serviceMonitors.Items),
		servicePodMappings:     workloads.MapServicesToPods(services, pods),
		serviceMonitorMappings: monitoringanalyzer.MapServiceMonitorsToServices(serviceMonitors, services),
		serviceMonitorPorts:    monitoringanalyzer.CheckServiceMonitorPorts(serviceMonitors, services),
	}, nil
}

func printReport(report report, verbose bool) {
	if !verbose {
		fmt.Println("k8s-monitor-debugger")
	}
	fmt.Printf("Cluster endpoint: %s\n", report.clusterEndpoint)
	fmt.Printf("Namespace: %s\n\n", report.namespace)

	printSummary(report)
	printProblems(report)
	printMappings(report)
	if verbose {
		printDetails(report)
	}
}

func printSummary(report report) {
	fmt.Println("Summary")
	fmt.Printf("- Services scanned: %d\n", len(report.services.Items))
	fmt.Printf("- Pods scanned: %d\n", len(report.pods.Items))
	fmt.Printf("- ServiceMonitors scanned: %d\n", report.serviceMonitorCount)
	fmt.Printf("- Service -> Pod matches: %d\n", servicePodMatchCount(report.servicePodMappings))
	fmt.Printf("- ServiceMonitors with matching Services: %d\n", serviceMonitorsWithMatchesCount(report.serviceMonitorMappings))
	fmt.Printf("- ServiceMonitors with no matching Services: %d\n", serviceMonitorsWithoutMatchesCount(report.serviceMonitorMappings))
	fmt.Printf("- Unsupported ServiceMonitor selectors: %d\n\n", unsupportedServiceMonitorSelectorCount(report.serviceMonitorMappings))
}

func printProblems(report report) {
	fmt.Println("Problems")
	fmt.Println()
	fmt.Println("ServiceMonitors with no matching Services")
	printNameList(serviceMonitorsWithoutMatches(report.serviceMonitorMappings))
	fmt.Println()
	fmt.Println("Services with no matching Pods")
	printNameList(servicesWithoutPods(report.servicePodMappings))
	fmt.Println()
}

func printMappings(report report) {
	fmt.Println("Mappings")
	fmt.Println()
	fmt.Println("Service -> Pod")
	servicePodMappingCount := 0
	for _, mapping := range report.servicePodMappings {
		for _, podName := range mapping.PodNames {
			fmt.Printf("- %s -> %s\n", mapping.ServiceName, podName)
			servicePodMappingCount++
		}
	}
	if servicePodMappingCount == 0 {
		fmt.Println("- none")
	}
	fmt.Println()
	fmt.Println("ServiceMonitor -> Service")
	serviceMonitorMappingCount := 0
	for _, mapping := range report.serviceMonitorMappings {
		for _, serviceName := range mapping.ServiceNames {
			fmt.Printf("- %s -> %s\n", mapping.ServiceMonitorName, serviceName)
			serviceMonitorMappingCount++
		}
	}
	if serviceMonitorMappingCount == 0 {
		fmt.Println("- none")
	}
	fmt.Println()
	fmt.Println("Healthy ServiceMonitor paths")
	printNameList(healthyServiceMonitorPaths(report.serviceMonitorPorts))
	fmt.Println()
}

func printDetails(report report) {
	fmt.Println("Details")
	fmt.Println()
	printServiceDetails(report.services)
	fmt.Println()
	printPodDetails(report.pods)
	fmt.Println()
	printServiceMonitorDetails(report.serviceMonitors)
}

func printServiceDetails(services *v1.ServiceList) {
	fmt.Println("Services")
	for _, service := range services.Items {
		fmt.Printf("- %s\n", service.Name)
		fmt.Printf("  namespace: %s\n", service.Namespace)
		fmt.Printf("  type: %s\n", service.Spec.Type)
		fmt.Printf("  selector: %s\n", formatStringMap(service.Spec.Selector))
		fmt.Printf("  labels: %s\n", formatStringMap(service.Labels))
		fmt.Println("  ports:")
		if len(service.Spec.Ports) == 0 {
			fmt.Println("    - none")
		}
		for _, port := range service.Spec.Ports {
			fmt.Printf("    - name=%s port=%d targetPort=%s\n", port.Name, port.Port, port.TargetPort.String())
		}
		fmt.Println()
	}
}

func printPodDetails(pods *v1.PodList) {
	fmt.Println("Pods")
	for _, pod := range pods.Items {
		fmt.Printf("- %s\n", pod.Name)
		fmt.Printf("  namespace: %s\n", pod.Namespace)
		fmt.Printf("  labels: %s\n", formatStringMap(pod.Labels))
		fmt.Println()
	}
}

func printServiceMonitorDetails(serviceMonitors *monitoringv1.ServiceMonitorList) {
	fmt.Println("ServiceMonitors")
	for _, serviceMonitor := range serviceMonitors.Items {
		fmt.Printf("- %s\n", serviceMonitor.Name)
		fmt.Printf("  namespace: %s\n", serviceMonitor.Namespace)
		fmt.Printf("  selector: %s\n", formatStringMap(serviceMonitor.Spec.Selector.MatchLabels))
		fmt.Println("  endpoints:")
		if len(serviceMonitor.Spec.Endpoints) == 0 {
			fmt.Println("    - none")
		}
		for _, endpoint := range serviceMonitor.Spec.Endpoints {
			fmt.Printf("    - %s\n", endpoint.Port)
		}
		fmt.Println()
	}
}

func printNameList(names []string) {
	if len(names) == 0 {
		fmt.Println("- none")
		return
	}

	for _, name := range names {
		fmt.Printf("- %s\n", name)
	}
}

func servicePodMatchCount(mappings []workloads.ServicePodMapping) int {
	count := 0
	for _, mapping := range mappings {
		count += len(mapping.PodNames)
	}
	return count
}

func serviceMonitorsWithMatchesCount(mappings []monitoringanalyzer.ServiceMonitorServiceMapping) int {
	count := 0
	for _, mapping := range mappings {
		if len(mapping.ServiceNames) > 0 {
			count++
		}
	}
	return count
}

func serviceMonitorsWithoutMatchesCount(mappings []monitoringanalyzer.ServiceMonitorServiceMapping) int {
	return len(serviceMonitorsWithoutMatches(mappings))
}

func unsupportedServiceMonitorSelectorCount(mappings []monitoringanalyzer.ServiceMonitorServiceMapping) int {
	count := 0
	for _, mapping := range mappings {
		if mapping.UnsupportedMatchExpression {
			count++
		}
	}
	return count
}

func serviceMonitorsWithoutMatches(mappings []monitoringanalyzer.ServiceMonitorServiceMapping) []string {
	names := []string{}
	for _, mapping := range mappings {
		if len(mapping.ServiceNames) == 0 {
			names = append(names, mapping.ServiceMonitorName)
		}
	}
	return names
}

func servicesWithoutPods(mappings []workloads.ServicePodMapping) []string {
	names := []string{}
	for _, mapping := range mappings {
		if !mapping.NoSelector && len(mapping.PodNames) == 0 {
			names = append(names, mapping.ServiceName)
		}
	}
	return names
}

func healthyServiceMonitorPaths(statuses []monitoringanalyzer.ServiceMonitorPortStatus) []string {
	names := []string{}
	for _, status := range statuses {
		if len(status.RogueServices) == 0 {
			names = append(names, status.ServiceMonitorName)
		}
	}
	return names
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "none"
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ", ")
}
