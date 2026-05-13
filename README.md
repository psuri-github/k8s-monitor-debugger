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
