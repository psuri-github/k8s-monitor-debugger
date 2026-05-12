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
- Prometheus Operator resources present for ServiceMonitor analysis

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

Default output is a compact report with:

- cluster endpoint and namespace
- scan summary counts
- problem sections for missing ServiceMonitor and Service matches
- Service → Pod and ServiceMonitor → Service mappings
- healthy ServiceMonitor paths

## Current limitations
- MatchExpressions are not yet supported
- Output is CLI text only

## Next steps
- Support MatchExpressions
- Add verbose output mode

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
