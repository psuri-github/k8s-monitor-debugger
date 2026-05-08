# k8s-monitor-debugger

A small Go-based Kubernetes debugging tool for understanding how monitoring resources connect inside a namespace.

## What this project does

Given a namespace, this tool currently helps answer:

- Which **Services** map to which **Pods**
- Which **ServiceMonitors** map to which **Services**
- Which **Services** have no matching Pods
- Which **ServiceMonitors** have no matching Services
- Which matched **ServiceMonitor -> Service** pairs have endpoint/port-name mismatches

This project is intentionally small and focused. It was built as a learning and debugging tool for Kubernetes resource relationships, especially around Prometheus Operator monitoring.

---

## Why I built this

I built this project to better understand:

- Kubernetes `client-go`
- how Services select Pods using label selectors
- how ServiceMonitors select Services using label selectors
- how ServiceMonitor endpoint ports relate to Service port names
- Go syntax and project structure in a real Kubernetes use case

---

## Current scope

### Usage

```bash
go run . -namespace monitoring
```

If `-namespace` is omitted, the tool defaults to `monitoring`.

### Implemented

- Connects to a Kubernetes cluster using kubeconfig
- Lists Services and Pods in a namespace
- Prints Service details:
  - name
  - namespace
  - selector
  - labels
  - type
  - ports
- Prints Pod details:
  - name
  - namespace
  - labels
- Builds **Service -> Pod** mappings using:
  - `Service.Spec.Selector`
  - `Pod.Labels`
- Lists ServiceMonitors in a namespace
- Prints ServiceMonitor details:
  - name
  - namespace
  - selector
  - endpoint ports
- Builds **ServiceMonitor -> Service** mappings using:
  - `ServiceMonitor.Spec.Selector.MatchLabels`
  - `Service.Labels`
- Extracts:
  - Service port names
  - ServiceMonitor endpoint port names
- Compares endpoint ports vs Service port names to flag likely mismatches

### Current limitations

- `MatchExpressions` are detected but not yet supported
- Output is CLI text only
- Code can still be refactored into cleaner analyzer packages

---

## Project structure

```text
k8s-monitor-debugger/
├── go.mod
├── go.sum
├── main.go
└── pkg/
    └── kube/
        └── client.go
```

`pkg/kube/client.go` contains both the standard Kubernetes client setup and the Prometheus Operator monitoring client setup.
