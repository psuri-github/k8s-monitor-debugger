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

## Current limitations
- MatchExpressions are not yet supported
- Output is CLI text only
- The codebase can still be refactored into cleaner analyzer packages

## Next steps
- Support MatchExpressions
- Improve reporting and output structure
- Split logic into dedicated analyzer packages
