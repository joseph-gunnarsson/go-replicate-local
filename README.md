# Go-Sim-Local

A lightweight, local orchestration and load balancing tool for Go microservices.

`go-replicate-local` allows you to simulate a distributed environment locally. It orchestrates multiple replicas of your services, manages their ports, and provides a Layer 7 Round-Robin load balancer to route traffic to them—all controlled via a terminal user interface (TUI).

## Features

*   **Local Orchestration**: Spin up multiple replicas of your Go services with a single config.
*   **Load Balancing**: Built-in HTTP Round-Robin load balancer.
*   **Process Management**:
    *   Automatic port assignment.
    *   Graceful shutdown of all services and replicas.
    *   **Process Group Isolation**: Ensures no zombie processes or stuck ports on exit.
*   **Terminal UI (TUI)**:
    *   Monitor logs in real-time.
    *   **Isolate**: Filter logs to view only a specific replica.
    *   **Kill**: Stop specific replicas to simulate failures.

## Installation

```bash
git clone https://github.com/joseph-gunnarsson/go-replicate-local.git
cd go-replicate-local
go build -o go-sim ./cmd/orchestrator
```

## Usage

1.  **Create a Services Configuration**:
    Create a `simulation.yaml` file (see [Example](examples/simulation.yaml)):

    ```yaml
    lb_port: 8079
    services:
      auth-service:
        name: "auth-service"
        path: "./examples/auth-service/main.go"
        start_port: 8083
        end_port: 8200
        replicas: 3
        route_prefix: "/auth"
    ```

2.  **Run the Orchestrator**:

    ```bash
    go run ./cmd/orchestrator --config simulation.yaml
    ```

3.  **Interact via TUI**:
    *   **Type commands** into the footer input.
    *   `list`: See running replicas.
    *   `isolate <name>`: View logs for just that replica (e.g., `isolate auth-service-1`).
    *   `showall`: View logs for all services.
    *   `kill <name>`: Kill a replica to test fault tolerance.
    *   `quit`: Shutdown everything and exit.

## Architecture

*   **Orchestrator**: Parses config and manages the lifecycle of service processes.
*   **Runner**: Wraps `go run`, handles process groups, and captures stdout/stderr.
*   **Load Balancer**: A reverse proxy that cycles requests to active backends.
*   **Interface**: A Bubble Tea-based TUI for control and monitoring.
