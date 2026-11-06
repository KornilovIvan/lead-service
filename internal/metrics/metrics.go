package metrics

import "github.com/prometheus/client_golang/prometheus"

// SyncJobsFailedTotal counts sync jobs that actually transitioned to failed.
var SyncJobsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "sync_jobs_failed_total",
	Help: "Total number of sync jobs that transitioned to failed.",
})

func init() {
	prometheus.MustRegister(SyncJobsFailedTotal)
}
