package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	monitoringanalyzer "k8s-monitor-debugger/pkg/analyzer/monitoring"
	"k8s-monitor-debugger/pkg/analyzer/workloads"
	"k8s-monitor-debugger/pkg/kube"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type report struct {
	clusterEndpoint           string
	namespace                 string
	services                  *v1.ServiceList
	pods                      *v1.PodList
	serviceMonitors           *monitoringv1.ServiceMonitorList
	serviceMonitorSkipped     bool
	serviceMonitorSkipReasons []string
	serviceMonitorCount       int
	servicePodMappings        []workloads.ServicePodMapping
	serviceMonitorMappings    []monitoringanalyzer.ServiceMonitorServiceMapping
	serviceMonitorPorts       []monitoringanalyzer.ServiceMonitorPortStatus
}

type jsonReport struct {
	ClusterEndpoint            string             `json:"clusterEndpoint"`
	Namespace                  string             `json:"namespace"`
	Summary                    jsonSummary        `json:"summary"`
	Problems                   jsonProblems       `json:"problems"`
	Mappings                   jsonMappings       `json:"mappings"`
	HealthyServiceMonitorPaths []string           `json:"healthyServiceMonitorPaths"`
	ServiceMonitorAnalysis     jsonAnalysisStatus `json:"serviceMonitorAnalysis"`
	Details                    *jsonDetails       `json:"details,omitempty"`
}

type jsonAnalysisStatus struct {
	Skipped bool     `json:"skipped"`
	Reasons []string `json:"reasons,omitempty"`
}

type jsonSummary struct {
	ServicesScanned                       int `json:"servicesScanned"`
	PodsScanned                           int `json:"podsScanned"`
	ServiceMonitorsScanned                int `json:"serviceMonitorsScanned"`
	ServicePodMatches                     int `json:"servicePodMatches"`
	ServiceMonitorsWithMatchingServices   int `json:"serviceMonitorsWithMatchingServices"`
	ServiceMonitorsWithNoMatchingServices int `json:"serviceMonitorsWithNoMatchingServices"`
	UnsupportedServiceMonitorSelectors    int `json:"unsupportedServiceMonitorSelectors"`
}

type jsonProblems struct {
	ServiceMonitorsWithNoMatchingServices []string `json:"serviceMonitorsWithNoMatchingServices"`
	ServicesWithNoMatchingPods            []string `json:"servicesWithNoMatchingPods"`
}

type jsonMappings struct {
	ServiceToPod            []jsonMapping `json:"serviceToPod"`
	ServiceMonitorToService []jsonMapping `json:"serviceMonitorToService"`
}

type jsonMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type jsonDetails struct {
	Services        []jsonService        `json:"services"`
	Pods            []jsonPod            `json:"pods"`
	ServiceMonitors []jsonServiceMonitor `json:"serviceMonitors"`
}

type jsonService struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	Selector  map[string]string `json:"selector"`
	Labels    map[string]string `json:"labels"`
	Ports     []jsonServicePort `json:"ports"`
}

type jsonServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort"`
}

type jsonPod struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

type jsonServiceMonitor struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Selector  map[string]string `json:"selector"`
	Endpoints []string          `json:"endpoints"`
}

func main() {
	namespace := flag.String("namespace", "monitoring", "Kubernetes namespace to inspect")
	verbose := flag.Bool("verbose", false, "Print detailed Services, Pods, and ServiceMonitors")
	output := flag.String("output", "text", "Output format: text or json")
	flag.Parse()

	if *namespace == "" {
		log.Fatal("namespace cannot be empty")
	}
	if *output != "text" && *output != "json" {
		log.Fatalf("unsupported output format %q: use text or json", *output)
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

	if *output == "json" {
		if err := printJSONReport(report, *verbose); err != nil {
			log.Fatalf("failed to write json output: %v", err)
		}
		return
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
		if isServiceMonitorCRDMissing(err) {
			return report{
				clusterEndpoint:       clusterEndpoint,
				namespace:             namespace,
				services:              services,
				pods:                  pods,
				serviceMonitors:       &monitoringv1.ServiceMonitorList{},
				serviceMonitorSkipped: true,
				serviceMonitorSkipReasons: []string{
					"ServiceMonitor CRD not found",
					"Prometheus Operator resources may not be installed in this cluster",
				},
				servicePodMappings: workloads.MapServicesToPods(services, pods),
			}, nil
		}
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
	printServiceMonitorSkipped(report)
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

func printServiceMonitorSkipped(report report) {
	if !report.serviceMonitorSkipped {
		return
	}

	fmt.Println("ServiceMonitor analysis skipped:")
	printNameList(report.serviceMonitorSkipReasons)
	fmt.Println()
}

func printProblems(report report) {
	fmt.Println("Problems")
	fmt.Println()
	if !report.serviceMonitorSkipped {
		fmt.Println("ServiceMonitors with no matching Services")
		printNameList(serviceMonitorsWithoutMatches(report.serviceMonitorMappings))
		fmt.Println()
	}
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
	if !report.serviceMonitorSkipped {
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
}

func printDetails(report report) {
	fmt.Println("Details")
	fmt.Println()
	printServiceDetails(report.services)
	fmt.Println()
	printPodDetails(report.pods)
	fmt.Println()
	if report.serviceMonitorSkipped {
		fmt.Println("ServiceMonitors")
		fmt.Println("ServiceMonitor analysis skipped:")
		printNameList(report.serviceMonitorSkipReasons)
		return
	}
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

func printJSONReport(report report, verbose bool) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(buildJSONReport(report, verbose))
}

func buildJSONReport(report report, verbose bool) jsonReport {
	jsonReport := jsonReport{
		ClusterEndpoint: report.clusterEndpoint,
		Namespace:       report.namespace,
		Summary: jsonSummary{
			ServicesScanned:                       len(report.services.Items),
			PodsScanned:                           len(report.pods.Items),
			ServiceMonitorsScanned:                report.serviceMonitorCount,
			ServicePodMatches:                     servicePodMatchCount(report.servicePodMappings),
			ServiceMonitorsWithMatchingServices:   serviceMonitorsWithMatchesCount(report.serviceMonitorMappings),
			ServiceMonitorsWithNoMatchingServices: serviceMonitorsWithoutMatchesCount(report.serviceMonitorMappings),
			UnsupportedServiceMonitorSelectors:    unsupportedServiceMonitorSelectorCount(report.serviceMonitorMappings),
		},
		Problems: jsonProblems{
			ServiceMonitorsWithNoMatchingServices: serviceMonitorsWithoutMatches(report.serviceMonitorMappings),
			ServicesWithNoMatchingPods:            servicesWithoutPods(report.servicePodMappings),
		},
		Mappings: jsonMappings{
			ServiceToPod:            jsonServicePodMappings(report.servicePodMappings),
			ServiceMonitorToService: jsonServiceMonitorMappings(report.serviceMonitorMappings),
		},
		HealthyServiceMonitorPaths: healthyServiceMonitorPaths(report.serviceMonitorPorts),
		ServiceMonitorAnalysis: jsonAnalysisStatus{
			Skipped: report.serviceMonitorSkipped,
			Reasons: report.serviceMonitorSkipReasons,
		},
	}

	if verbose {
		jsonReport.Details = &jsonDetails{
			Services:        jsonServices(report.services),
			Pods:            jsonPods(report.pods),
			ServiceMonitors: jsonServiceMonitors(report.serviceMonitors),
		}
	}

	return jsonReport
}

func isServiceMonitorCRDMissing(err error) bool {
	return apierrors.IsNotFound(err)
}

func jsonServicePodMappings(mappings []workloads.ServicePodMapping) []jsonMapping {
	jsonMappings := []jsonMapping{}
	for _, mapping := range mappings {
		for _, podName := range mapping.PodNames {
			jsonMappings = append(jsonMappings, jsonMapping{
				From: mapping.ServiceName,
				To:   podName,
			})
		}
	}
	return jsonMappings
}

func jsonServiceMonitorMappings(mappings []monitoringanalyzer.ServiceMonitorServiceMapping) []jsonMapping {
	jsonMappings := []jsonMapping{}
	for _, mapping := range mappings {
		for _, serviceName := range mapping.ServiceNames {
			jsonMappings = append(jsonMappings, jsonMapping{
				From: mapping.ServiceMonitorName,
				To:   serviceName,
			})
		}
	}
	return jsonMappings
}

func jsonServices(services *v1.ServiceList) []jsonService {
	jsonServices := []jsonService{}
	for _, service := range services.Items {
		ports := []jsonServicePort{}
		for _, port := range service.Spec.Ports {
			ports = append(ports, jsonServicePort{
				Name:       port.Name,
				Port:       port.Port,
				TargetPort: port.TargetPort.String(),
			})
		}

		jsonServices = append(jsonServices, jsonService{
			Name:      service.Name,
			Namespace: service.Namespace,
			Type:      string(service.Spec.Type),
			Selector:  service.Spec.Selector,
			Labels:    service.Labels,
			Ports:     ports,
		})
	}
	return jsonServices
}

func jsonPods(pods *v1.PodList) []jsonPod {
	jsonPods := []jsonPod{}
	for _, pod := range pods.Items {
		jsonPods = append(jsonPods, jsonPod{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		})
	}
	return jsonPods
}

func jsonServiceMonitors(serviceMonitors *monitoringv1.ServiceMonitorList) []jsonServiceMonitor {
	jsonServiceMonitors := []jsonServiceMonitor{}
	for _, serviceMonitor := range serviceMonitors.Items {
		endpoints := []string{}
		for _, endpoint := range serviceMonitor.Spec.Endpoints {
			endpoints = append(endpoints, endpoint.Port)
		}

		jsonServiceMonitors = append(jsonServiceMonitors, jsonServiceMonitor{
			Name:      serviceMonitor.Name,
			Namespace: serviceMonitor.Namespace,
			Selector:  serviceMonitor.Spec.Selector.MatchLabels,
			Endpoints: endpoints,
		})
	}
	return jsonServiceMonitors
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
