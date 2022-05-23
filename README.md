# Ktoolhu

Ktoolhu is Kubernetes tool to do different weird things with K8s API.

## Features

- `perf-configmaps` - Create or update configmaps multiple times to create load
- `restart-all`     - Restart all workload in cluster or namespace
- `secret`          - Encode or decode k8s secret from stdin to stdout (yaml or json)
- `terminating-ns`  - List or delete terminating namespaces


## Usage

```shell
% ktoolhu -h
Ktoolhu is tool to do various weird stuff with Kubernetes API

Usage:
  ktoolhu [command]

Available Commands:
  help            Help about any command
  perf-configmaps Create or update configmaps multiple times to create load
  restart-all     Restart all workload in cluster or namespace
  secret          Encode or decode k8s secret from stdin to stdout (yaml or json)
  terminating-ns  List or delete terminating namespaces

Flags:
  -h, --help                help for ktoolhu
      --kubeconfig string   absolute path to the kubeconfig file (default "/Users/JP25060/.kube/config")
  -n, --namespace string    namespace (default "ktoolhu")

Use "ktoolhu [command] --help" for more information about a command.
```

## Build

```shell
go build -o ktoolhu main.go
```
