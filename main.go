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

// Metric definitions
var (
	maintenanceStartMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_start_timestamp",
			Help: "Scheduled maintenance window start time (Unix timestamp)",
		},
		[]string{"server_id", "name", "zone", "machine_type", "maintenance_window_readable"},
	)

	maintenanceEndMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_end_timestamp",
			Help: "Scheduled maintenance window end time (Unix timestamp)",
		},
		[]string{"server_id", "name", "zone", "machine_type", "maintenance_window_readable"},
	)

	maintenanceStatusMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stackit_server_maintenance_status",
			Help: "Status of the maintenance window (one-hot encoded: PLANNED or ONGOING = 1, others = 0)",
		},
		[]string{"server_id", "name", "zone", "machine_type", "maintenance_window_readable", "status"},
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

	// Set up HTTP server
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

// startScraper runs a ticker to periodically update metrics
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

// updateMetrics fetches server data and sets Prometheus metrics
func updateMetrics(client *iaas.APIClient, projectID string) error {
	servers, err := client.ListServers(context.Background(), projectID).Execute()
	if err != nil {
		return err
	}

	log.Printf("Updating metrics for %d servers", len(*servers.Items))
	for _, srv := range *servers.Items {
		if srv.Id == nil || srv.Name == nil || srv.AvailabilityZone == nil || srv.MachineType == nil {
			continue
		}

		var readableLabel string
		var unixStart, unixEnd float64

		mw := srv.MaintenanceWindow
		if mw != nil && mw.StartsAt != nil && mw.EndsAt != nil {
			// Label = full window in RFC3339 (from - to)
			readableLabel = mw.StartsAt.UTC().Format(time.RFC3339) + "_to_" + mw.EndsAt.UTC().Format(time.RFC3339)
			unixStart = float64(mw.StartsAt.UTC().Unix())
			unixEnd = float64(mw.EndsAt.UTC().Unix())
		} else if mw != nil && mw.StartsAt != nil {
			readableLabel = mw.StartsAt.UTC().Format(time.RFC3339)
			unixStart = float64(mw.StartsAt.UTC().Unix())
			unixEnd = 0
		} else {
			readableLabel = "unknown"
			unixStart = 0
			unixEnd = 0
		}

		baseLabels := []string{
			*srv.Id,
			*srv.Name,
			*srv.AvailabilityZone,
			*srv.MachineType,
			readableLabel,
		}

		// Update metrics
		maintenanceStartMetric.WithLabelValues(baseLabels...).Set(unixStart)
		maintenanceEndMetric.WithLabelValues(baseLabels...).Set(unixEnd)

		if mw != nil && mw.Status != nil {
			updateStatusMetric(*mw.Status, baseLabels)
		} else {
			clearStatusMetrics(baseLabels)
		}
	}
	return nil
}

// updateStatusMetric sets status labels (PLANNED or ONGOING)
func updateStatusMetric(status string, baseLabels []string) {
	switch status {
	case statusPlanned, statusOngoing:
		for _, s := range []string{statusPlanned, statusOngoing} {
			val := 0.0
			if status == s {
				val = 1.0
			}
			maintenanceStatusMetric.WithLabelValues(append(baseLabels, s)...).Set(val)
		}
	default:
		clearStatusMetrics(baseLabels)
	}
}

// clearStatusMetrics resets known status labels to 0
func clearStatusMetrics(baseLabels []string) {
	for _, s := range []string{statusPlanned, statusOngoing} {
		maintenanceStatusMetric.WithLabelValues(append(baseLabels, s)...).Set(0)
	}
}
