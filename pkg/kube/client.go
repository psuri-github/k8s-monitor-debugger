package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/rest"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
)

func GetClient() (*kubernetes.Clientset, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, err
	}
	fmt.Println("API SERVER ENDPOINT : " + config.Host)
	return kubernetes.NewForConfig(config)
}

func GetMonitoringClient() (*monitoringclient.Clientset, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, err
	}
	return monitoringclient.NewForConfig(config)
}

func GetConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}
