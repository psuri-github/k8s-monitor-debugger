package workloads

import (
	"k8s-monitor-debugger/pkg/analyzer/labels"

	v1 "k8s.io/api/core/v1"
)

type ServicePodMapping struct {
	ServiceName string
	PodNames    []string
	NoSelector  bool
}

func MapServicesToPods(services *v1.ServiceList, pods *v1.PodList) []ServicePodMapping {
	mappings := []ServicePodMapping{}

	for _, service := range services.Items {
		mapping := ServicePodMapping{
			ServiceName: service.Name,
			NoSelector:  len(service.Spec.Selector) == 0,
		}

		if mapping.NoSelector {
			mappings = append(mappings, mapping)
			continue
		}

		for _, pod := range pods.Items {
			if labels.Match(service.Spec.Selector, pod.Labels) {
				mapping.PodNames = append(mapping.PodNames, pod.Name)
			}
		}

		mappings = append(mappings, mapping)
	}

	return mappings
}
