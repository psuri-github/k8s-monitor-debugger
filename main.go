package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"slices"

	"k8s-monitor-debugger/pkg/kube"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
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
	for _, sm := range serviceMonitors.Items {
		if len(sm.Spec.Selector.MatchLabels) == 0 {
			if len(sm.Spec.Selector.MatchExpressions) > 0 {
				fmt.Println(sm.Name + " uses MatchExpressions; not supported yet")
			} else {
				fmt.Println(sm.Name + " : no selector")
			}
			continue
		}
		foundMatch := false
		for _, s := range services.Items {
			if labelsMatch(sm.Spec.Selector.MatchLabels, s.Labels) {
				fmt.Println(sm.Name + " : " + s.Name)
				foundMatch = true
			}
		}
		if !foundMatch {
			fmt.Println(sm.Name + " : No matching Service Found")
		}
	}
	fmt.Println("############################################################")
	fmt.Println()
	smToendpointMap := fetchEndpointsForServiceMonitors(serviceMonitors)
	smToservicesMap := fetchServicesForServiceMonitors(serviceMonitors, services)
	svcToPortMap := fetchPortsForServices(services)
	for sm, svcList := range smToservicesMap {
		roguesvc := []string{}
		endpoints := smToendpointMap[sm]
		for _, svc := range svcList {
			ports := svcToPortMap[svc]
			if !sameIgnoringOrder(endpoints, ports) {
				roguesvc = append(roguesvc, svc)
			}
		}
		if len(roguesvc) == 0 {
			fmt.Println("sm : GREEN")
		} else {
			fmt.Println("sm : RED ROGUE SVCS ", roguesvc)
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
	for _, s := range services.Items {
		if len(s.Spec.Selector) == 0 {
			fmt.Println(s.Name + " : no selector")
			continue
		}
		foundMatch := false
		for _, p := range pods.Items {
			if labelsMatch(s.Spec.Selector, p.Labels) {
				fmt.Println(s.Name + " : " + p.Name)
				foundMatch = true
			}
		}
		if !foundMatch {
			fmt.Println(s.Name + " : No matching POD found")
		}
	}
	fmt.Println("############################################################")
	fmt.Println()
}

func labelsMatch(selector map[string]string, labels map[string]string) bool {
	for key, expectedValue := range selector {
		actualValue, exists := labels[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}

func fetchPortsForServices(services *v1.ServiceList) map[string][]string {
	svcToPortMap := make(map[string][]string)
	for _, s := range services.Items {
		ports := []string{}
		for _, port := range s.Spec.Ports {
			ports = append(ports, port.Name)
		}
		svcToPortMap[s.Name] = ports
	}
	return svcToPortMap
}

func fetchEndpointsForServiceMonitors(serviceMonitors *monitoringv1.ServiceMonitorList) map[string][]string {
	smToendpointsMap := make(map[string][]string)
	for _, sm := range serviceMonitors.Items {
		endpoints := []string{}
		for _, endpoint := range sm.Spec.Endpoints {
			endpoints = append(endpoints, endpoint.Port)
		}
		smToendpointsMap[sm.Name] = endpoints
	}
	return smToendpointsMap
}

func fetchServicesForServiceMonitors(serviceMonitors *monitoringv1.ServiceMonitorList, services *v1.ServiceList) map[string][]string {
	smToservicesMap := make(map[string][]string)
	for _, sm := range serviceMonitors.Items {
		svcs := []string{}
		for _, s := range services.Items {
			if labelsMatch(sm.Spec.Selector.MatchLabels, s.Labels) {
				svcs = append(svcs, s.Name)
			}
		}
		if len(svcs) == 0 {
			fmt.Println(sm.Name + " : No matching Service Found")
		} else {
			smToservicesMap[sm.Name] = svcs
		}
	}
	return smToservicesMap
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
