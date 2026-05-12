package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	monitoringanalyzer "k8s-monitor-debugger/pkg/analyzer/monitoring"
	"k8s-monitor-debugger/pkg/analyzer/workloads"
	"k8s-monitor-debugger/pkg/kube"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	namespace := flag.String("namespace", "monitoring", "Kubernetes namespace to inspect")
	flag.Parse()

	if *namespace == "" {
		log.Fatal("namespace cannot be empty")
	}

	processKubeClients(*namespace)
	processMonitoringClients(*namespace)
}

func processMonitoringClients(namespace string) {
	monitorclient, err := kube.GetMonitoringClient()
	if err != nil {
		log.Fatalf("failed to create monitoring client: %v", err)
	}
	serviceMonitors, err := monitorclient.MonitoringV1().ServiceMonitors(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list serviceMonitors in Namespace %q: %v", namespace, err)
	}
	for _, sm := range serviceMonitors.Items {
		fmt.Printf("ServiceMonitor Name - %s\n", sm.Name)
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("ServiceMonitor NameSpace - %s\n", sm.Namespace)
		fmt.Printf("ServiceMonitor Selector - %v\n", sm.Spec.Selector)
		fmt.Printf("ServiceMonitor EndPoints\n")
		fmt.Println()
		fmt.Println("------------------------------------------------------------")
		fmt.Println("------------------------------------------------------------")
		for _, endpoint := range sm.Spec.Endpoints {
			fmt.Printf("\t EndPoint Port - %s\n", endpoint.Port)
		}
		fmt.Println("------------------------------------------------------------")
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
	}

	client, err := kube.GetClient()
	if err != nil {
		log.Fatalf("failed to create kube client: %v", err)
	}
	services, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list services in namespace %q: %v", namespace, err)
	}

	fmt.Println("############################################################")

	fmt.Println("ServiceMonitor -> Service Mapping")
	for _, mapping := range monitoringanalyzer.MapServiceMonitorsToServices(serviceMonitors, services) {
		if mapping.NoSelector {
			if mapping.UnsupportedMatchExpression {
				fmt.Println(mapping.ServiceMonitorName + " uses MatchExpressions; not supported yet")
			} else {
				fmt.Println(mapping.ServiceMonitorName + " : no selector")
			}
			continue
		}
		for _, serviceName := range mapping.ServiceNames {
			fmt.Println(mapping.ServiceMonitorName + " : " + serviceName)
		}
		if len(mapping.ServiceNames) == 0 {
			fmt.Println(mapping.ServiceMonitorName + " : No matching Service Found")
		}
	}
	fmt.Println("############################################################")
	fmt.Println()
	for _, status := range monitoringanalyzer.CheckServiceMonitorPorts(serviceMonitors, services) {
		if len(status.RogueServices) == 0 {
			fmt.Println(status.ServiceMonitorName + " : GREEN")
		} else {
			fmt.Println(status.ServiceMonitorName+" : RED ROGUE SVCS ", status.RogueServices)
		}
	}
}

func processKubeClients(namespace string) {
	client, err := kube.GetClient()
	if err != nil {
		log.Fatalf("failed to create kube client: %v", err)
	}

	services, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list services in namespace %q: %v", namespace, err)
	}

	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("failed to list pods in namespace %q: %v", namespace, err)
	}

	fmt.Printf("Namespace: %s\n\n", namespace)

	fmt.Println("Services:")
	fmt.Println("************************************************************")
	for _, s := range services.Items {
		fmt.Printf("Service Name - %s\n", s.Name)
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("Service NameSpace - %s\n", s.Namespace)
		fmt.Printf("Service Selector - %v\n", s.Spec.Selector)
		fmt.Printf("Service Type - %s\n", s.Spec.Type)
		fmt.Printf("Service Labels - %v\n", s.Labels)
		fmt.Printf("Service Ports\n")
		fmt.Println()
		fmt.Println("------------------------------------------------------------")
		fmt.Println("------------------------------------------------------------")
		for _, port := range s.Spec.Ports {
			fmt.Printf("\t Name : %s, Port : %d, TargetPort : %s\n", port.Name, port.Port, port.TargetPort.String())
		}
		fmt.Println("------------------------------------------------------------")
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
	}

	fmt.Println("\nPods:")
	fmt.Println("************************************************************")
	for _, p := range pods.Items {
		fmt.Printf("POD Name - %s\n", p.Name)
		fmt.Println("------------------------------------------------------------")
		fmt.Printf("POD Namespace - %s\n", p.Namespace)
		fmt.Printf("POD Labels - %v\n", p.Labels)
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
	}

	fmt.Println("############################################################")

	fmt.Println("Service -> POD Mapping")
	for _, mapping := range workloads.MapServicesToPods(services, pods) {
		if mapping.NoSelector {
			fmt.Println(mapping.ServiceName + " : no selector")
			continue
		}
		for _, podName := range mapping.PodNames {
			fmt.Println(mapping.ServiceName + " : " + podName)
		}
		if len(mapping.PodNames) == 0 {
			fmt.Println(mapping.ServiceName + " : No matching POD found")
		}
	}
	fmt.Println("############################################################")
	fmt.Println()
}
