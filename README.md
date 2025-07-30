# STACKIT Server Maintenance Exporter

This is a Prometheus exporter that exposes maintenance schedule metrics for STACKIT servers.  
Metrics are collected from the STACKIT IaaS API and exposed over an HTTP endpoint for scraping by Prometheus.

## Exported Metrics

| Metric Name                                | Description                                                                 |
|--------------------------------------------|-----------------------------------------------------------------------------|
| stackit_server_maintenance_start_timestamp | Maintenance window start time in Unix timestamp (UTC)                       |
| stackit_server_maintenance_end_timestamp   | Maintenance window end time in Unix timestamp (UTC)                         |
| stackit_server_maintenance_status          | One-hot encoded metric to indicate maintenance status: PLANNED or ONGOING   |

Each metric includes the following labels:

- server_id: The server's unique ID
- name: The name of the server
- zone: The availability zone of the server
- machine_type: The machine type of the server
- maintenance_window_readable: A human-readable representation of the maintenance window  
  Example: 2024-06-28T01:00:00Z_to_2024-06-28T02:00:00Z
- status: Only for stackit_server_maintenance_status — PLANNED or ONGOING

This allows easy alerting and inspection in dashboards by window time.

## Running the Exporter

Before starting the exporter, set the necessary environment variables for authenticating against the STACKIT IaaS API.  
These environment variables are described in the STACKIT Go SDK repository:  
🔗 https://github.com/stackitcloud/stackit-sdk-go#authentication

You must also set the STACKIT project to query servers from:

```bash
STACKIT_PROJECT_ID=your-project-id
```

## Endpoints

| Path      | Description                 |
|-----------|-----------------------------|
| /metrics  | Exposes Prometheus metrics  |
| /healthz  | Health check (HTTP 200 OK)  |

## Example Prometheus Alert: Notification on Maintenance Change

This alert triggers when a maintenance window start time is changed for any server (in its last 10 minutes):

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
          summary: "Maintenance window changed for server {{ $labels.server_id }}"
          description: >
            The maintenance schedule changed for server {{ $labels.name }} ({{ $labels.server_id }})
            in zone {{ $labels.zone }}.
            New maintenance window: {{ $labels.maintenance_window_readable }}.
```