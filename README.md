# Ktoolhu

Ktoolhu is Kubernetes tool to do different weird things with K8s API.

## Features

- `group-resources` - Show resources for each group-version (to find groups without resources)
- `perf-configmaps` - Create or update configmaps multiple times to create load
- `restart-all`     - Restart all workload in cluster or namespace

## Usage

```shell
% ktoolhu -h
Ktoolhu is tool to do various weird stuff with Kubernetes API

Usage:
  ktoolhu [command]

Available Commands:
  group-resources Show resources for each group-version (to find groups without resources)
  help            Help about any command
  perf-configmaps Create or update configmaps multiple times to create load
  restart-all     Restart all workload in cluster or namespace

Flags:
  -h, --help                help for ktoolhu
      --kubeconfig string   absolute path to the kubeconfig file (default "/home/mansur/.kube/config")
  -n, --namespace string    namespace (default "ktoolhu")

Use "ktoolhu [command] --help" for more information about a command.
```

## Build

```shell
go build -o ktoolhu main.go
```
