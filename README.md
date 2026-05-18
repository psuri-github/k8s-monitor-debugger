# k8s-monitor-debugger: Kubernetes Observability Wiring Debugger

`k8s-monitor-debugger` is a Go-based CLI tool for debugging how monitoring is wired inside a Kubernetes namespace.

It focuses on a practical failure mode: Prometheus scraping often breaks not because Prometheus is down, but because **Services, Pods, and ServiceMonitors do not line up the way operators expect**.

## What it checks

- Service → Pod mappings
- ServiceMonitor → Service mappings
- Services with no matching Pods
- ServiceMonitors with no matching Services
- Port-name mismatches between ServiceMonitors and Services

## Why it is useful

This tool helps debug issues such as:

- a Service selector not matching the intended Pods
- a ServiceMonitor selector not matching the intended Service
- a ServiceMonitor endpoint port not matching a named Service port

Instead of checking each resource manually, it helps surface how the monitoring path is actually wired.

## Requirements

- Access to a Kubernetes cluster through `kubeconfig`
- Read access to the target namespace
- Prometheus Operator resources present for ServiceMonitor analysis. If the ServiceMonitor CRD is missing, ServiceMonitor analysis is skipped and Service/Pod analysis still runs.

## Usage

Run with the default namespace (`monitoring`):

```bash
go run .
```
Or specify a namespace:
```bash
go run . -namespace monitoring
```
If no namespace is provided, the tool defaults to monitoring.

Print detailed resource information with verbose mode:

```bash
go run . -namespace monitoring -verbose
```

Print machine-readable JSON:

```bash
go run . -namespace monitoring -output json
```

Include detailed resource information in JSON:

```bash
go run . -namespace monitoring -output json -verbose
```

Default output is a compact report with:

- cluster endpoint and namespace
- scan summary counts
- problem sections for missing ServiceMonitor and Service matches
- Service → Pod and ServiceMonitor → Service mappings
- healthy ServiceMonitor paths

Verbose output includes the same report plus detailed Services, Pods, and ServiceMonitors.
JSON output uses the same report data in a structured format.

## Sample output

Compact text mode:

```text
k8s-monitor-debugger
Cluster endpoint: https://192.168.49.2:8443
Namespace: monitoring

Summary
- Services scanned: 4
- Pods scanned: 4
- ServiceMonitors scanned: 5
- Service -> Pod matches: 4
- ServiceMonitors with matching Services: 3
- ServiceMonitors with no matching Services: 2
- Unsupported ServiceMonitor selectors: 0

Problems

ServiceMonitors with no matching Services
- kube-prom-kube-prometheus-apiserver
- kube-prom-kube-prometheus-kubelet

Services with no matching Pods
- none

Mappings

Service -> Pod
- kube-prom-grafana -> kube-prom-grafana-6f4d9f6877-qc4v4
- kube-prom-kube-prometheus-operator -> kube-prom-kube-prometheus-operator-675647787d-l7589
- kube-prom-kube-state-metrics -> kube-prom-kube-state-metrics-f8b5ffb78-dqlgf
- kube-prom-prometheus-node-exporter -> kube-prom-prometheus-node-exporter-7g89t

ServiceMonitor -> Service
- kube-prom-grafana -> kube-prom-grafana
- kube-prom-kube-state-metrics -> kube-prom-kube-state-metrics
- kube-prom-prometheus-node-exporter -> kube-prom-prometheus-node-exporter

Healthy ServiceMonitor paths
- kube-prom-grafana
- kube-prom-kube-state-metrics
- kube-prom-prometheus-node-exporter
```

JSON mode:

```json
{
  "clusterEndpoint": "https://192.168.49.2:8443",
  "namespace": "monitoring",
  "summary": {
    "servicesScanned": 4,
    "podsScanned": 4,
    "serviceMonitorsScanned": 5,
    "servicePodMatches": 4,
    "serviceMonitorsWithMatchingServices": 3,
    "serviceMonitorsWithNoMatchingServices": 2,
    "unsupportedServiceMonitorSelectors": 0
  },
  "problems": {
    "serviceMonitorsWithNoMatchingServices": [
      "kube-prom-kube-prometheus-apiserver",
      "kube-prom-kube-prometheus-kubelet"
    ],
    "servicesWithNoMatchingPods": []
  },
  "mappings": {
    "serviceToPod": [
      {
        "from": "kube-prom-grafana",
        "to": "kube-prom-grafana-6f4d9f6877-qc4v4"
      },
      {
        "from": "kube-prom-kube-state-metrics",
        "to": "kube-prom-kube-state-metrics-f8b5ffb78-dqlgf"
      }
    ],
    "serviceMonitorToService": [
      {
        "from": "kube-prom-grafana",
        "to": "kube-prom-grafana"
      },
      {
        "from": "kube-prom-kube-state-metrics",
        "to": "kube-prom-kube-state-metrics"
      }
    ]
  },
  "healthyServiceMonitorPaths": [
    "kube-prom-grafana",
    "kube-prom-kube-state-metrics",
    "kube-prom-prometheus-node-exporter"
  ],
  "serviceMonitorAnalysis": {
    "skipped": false
  }
}
```

If the ServiceMonitor CRD is not installed, the tool prints:

```text
ServiceMonitor analysis skipped:
- ServiceMonitor CRD not found
- Prometheus Operator resources may not be installed in this cluster
```

## Exit status

- `0`: successful run, no issues found
- `1`: successful run, but mismatches, warnings, or issues were found
- `2`: tool or infrastructure failure prevented meaningful analysis

## Current limitations
- MatchExpressions are not yet supported

## Next steps
- Support MatchExpressions

## Project structure

```text
k8s-monitor-debugger/
├── main.go
├── pkg/
│   ├── analyzer/
│   │   ├── labels/
│   │   ├── monitoring/
│   │   └── workloads/
│   └── kube/
└── README.md
```

## Release
Pre-release artifacts for v0.1.0 are available under GitHub Releases.

Quick start
1. Download the archive for your platform
2. Extract the binary
3. Run:
   ./kubectl-mondbg -namespace monitoring


## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
