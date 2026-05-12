package workloads

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMapServicesToPods(t *testing.T) {
	services := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "api"},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{"app": "api"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "external"},
			},
		},
	}
	pods := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "api-1",
					Labels: map[string]string{"app": "api"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "web-1",
					Labels: map[string]string{"app": "web"},
				},
			},
		},
	}

	mappings := MapServicesToPods(services, pods)

	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].ServiceName != "api" || len(mappings[0].PodNames) != 1 || mappings[0].PodNames[0] != "api-1" {
		t.Fatalf("expected api service to map to api-1, got %#v", mappings[0])
	}
	if mappings[1].ServiceName != "external" || !mappings[1].NoSelector {
		t.Fatalf("expected external service to be marked as no selector, got %#v", mappings[1])
	}
}
