package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/stackitcloud/stackit-sdk-go/core/config"
	"github.com/stackitcloud/stackit-sdk-go/services/iaas"
)

const (
	scrapeInterval = 60 * time.Second
	statusPlanned  = "PLANNED"
	statusOngoing  = "ONGOING"
)

var (
	maintenanceStartMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_start_timestamp",
			Help: "Scheduled maintenance window start time (Unix timestamp)",
		},
		[]string{"server_id", "name", "zone", "machine_type"},
	)

	maintenanceEndMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_end_timestamp",
			Help: "Scheduled maintenance window end time (Unix timestamp)",
		},
		[]string{"server_id", "name", "zone", "machine_type"},
	)

	maintenanceStatusMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_status",
			Help: "Status of the maintenance window (one-hot encoded: PLANNED or ONGOING = 1, others = 0)",
		},
		[]string{"server_id", "name", "zone", "machine_type", "status"},
	)
)

func main() {
	log.Println("Starting stackit-maintenance-exporter on port 8080")

	projectID := os.Getenv("STACKIT_PROJECT_ID")
	if projectID == "" {
		log.Fatal("STACKIT_PROJECT_ID environment variable is not set")
	}

	client, err := iaas.NewAPIClient(config.WithRegion("eu01"))
	if err != nil {
		log.Fatalf("Failed to initialize STACKIT client: %v", err)
	}

	prometheus.MustRegister(maintenanceStartMetric, maintenanceEndMetric, maintenanceStatusMetric)

	go startScraper(client, projectID)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func startScraper(client *iaas.APIClient, projectID string) {
	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()

	for {
		if err := updateMetrics(client, projectID); err != nil {
			log.Printf("Error updating metrics: %v", err)
		}
		<-ticker.C
	}
}

func updateMetrics(client *iaas.APIClient, projectID string) error {
	servers, err := client.ListServers(context.Background(), projectID).Execute()
	if err != nil {
		return err
	}

	for _, srv := range *servers.Items {
		if srv.Id == nil || srv.Name == nil || srv.AvailabilityZone == nil || srv.MachineType == nil {
			continue
		}

		labels := []string{*srv.Id, *srv.Name, *srv.AvailabilityZone, *srv.MachineType}

		if mw := srv.MaintenanceWindow; mw != nil {
			// Use actual values if available; else use 0
			if mw.StartsAt != nil {
				maintenanceStartMetric.WithLabelValues(labels...).Set(float64(mw.StartsAt.UTC().Unix()))
			} else {
				maintenanceStartMetric.WithLabelValues(labels...).Set(0)
			}

			if mw.EndsAt != nil {
				maintenanceEndMetric.WithLabelValues(labels...).Set(float64(mw.EndsAt.UTC().Unix()))
			} else {
				maintenanceEndMetric.WithLabelValues(labels...).Set(0)
			}

			if mw.Status != nil {
				updateStatusMetric(*mw.Status, labels)
			} else {
				// If no status, set both known statuses to 0
				clearStatusMetrics(labels)
			}
		} else {
			// No maintenance window — set all metrics to 0
			maintenanceStartMetric.WithLabelValues(labels...).Set(0)
			maintenanceEndMetric.WithLabelValues(labels...).Set(0)
			clearStatusMetrics(labels)
		}
	}

	return nil
}

func updateStatusMetric(status string, baseLabels []string) {
	switch status {
	case statusPlanned, statusOngoing:
		for _, s := range []string{statusPlanned, statusOngoing} {
			value := 0.0
			if status == s {
				value = 1.0
			}
			maintenanceStatusMetric.WithLabelValues(append(baseLabels, s)...).Set(value)
		}
	default:
		// Unknown status — set all known statuses to 0
		clearStatusMetrics(baseLabels)
	}
}

func clearStatusMetrics(baseLabels []string) {
	for _, s := range []string{statusPlanned, statusOngoing} {
		maintenanceStatusMetric.WithLabelValues(append(baseLabels, s)...).Set(0)
	}
}
