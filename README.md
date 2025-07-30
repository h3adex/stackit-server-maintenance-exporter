# STACKIT Server Maintenance Exporter

This is a Prometheus exporter that exposes maintenance schedule metrics for STACKIT servers.
Metrics are collected from the STACKIT IaaS API and exposed over an HTTP endpoint for scraping by Prometheus.

## Exported Metrics

| Metric Name                                | Description                                                               |
|--------------------------------------------|---------------------------------------------------------------------------|
| stackit_server_maintenance_start_timestamp | Maintenance window start time (Unix timestamp)                            |
| stackit_server_maintenance_end_timestamp   | Maintenance window end time (Unix timestamp)                              |
| stackit_server_maintenance_status          | One-hot encoded metric indicating maintenance status (PLANNED or ONGOING) |

Each metric includes the following labels:  
server_id, name, zone, machine_type, and status (for stackit_server_maintenance_status).

## Running the Exporter

Before starting the exporter, set the necessary environment variables for authenticating against the STACKIT IaaS API.  
These environment variables are described in the STACKIT Go SDK repository:
https://github.com/stackitcloud/stackit-sdk-go#authentication

Additionally, set the following environment variable to specify your STACKIT project:

```bash
STACKIT_PROJECT_ID=your-project-id
```

## Endpoints

| Path      | Description                        |
|-----------|------------------------------------|
| /metrics  | Prometheus metrics endpoint        |
| /healthz  | Health check endpoint              |

## Example Prometheus Alert: Notification on Maintenance Change

This alert triggers once when the maintenance start time is changed for any virtual machine:

```yaml
groups:
  - name: stackit_maintenance_alerts
    rules:
      - alert: MaintenanceWindowChanged
        expr: changes(stackit_server_maintenance_start_timestamp[10m]) > 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "Maintenance changed for {{ $labels.server_id }}"
          description: >
            The maintenance start time changed or was added for server:
            {{ $labels.name }} ({{ $labels.server_id }}) in zone {{ $labels.zone }}.
```