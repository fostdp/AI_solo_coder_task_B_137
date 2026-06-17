package metrics

import (
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	SensorPacketsTotal    prometheus.Counter
	SensorPacketsInvalid  prometheus.Counter
	SimulationsTotal      prometheus.Counter
	PenetrationsTotal     prometheus.Counter
	AlertsTotal           prometheus.CounterVec
	DBInsertsTotal        prometheus.CounterVec
	DBInsertErrorsTotal   prometheus.CounterVec
	MQTTMessagesTotal     prometheus.Counter
	MQTTReconnectsTotal   prometheus.Counter

	SimDurationSeconds    prometheus.Histogram
	PenDurationSeconds    prometheus.Histogram
	UDPSizeBytes          prometheus.Histogram
	ImpactVelocity        prometheus.Histogram
	PenetrationDepth      prometheus.Histogram

	ActiveSimulations     prometheus.Gauge
	ActivePenetrations    prometheus.Gauge
	PendingSensorCount    prometheus.Gauge
	PendingSimCount       prometheus.Gauge
	PendingPenCount       prometheus.Gauge

	UpGauge               prometheus.Gauge
	StartTime             time.Time

	sensorSeq   uint64
	simSeq      uint64
	penSeq      uint64
	alertSeq    uint64
}

func New() *Metrics {
	m := &Metrics{
		SensorPacketsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "udp",
			Name:      "packets_total",
			Help:      "Total UDP sensor packets received",
		}),
		SensorPacketsInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "udp",
			Name:      "packets_invalid_total",
			Help:      "Total invalid UDP sensor packets rejected by validation",
		}),
		SimulationsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "sim",
			Name:      "runs_total",
			Help:      "Total ballistic simulations completed",
		}),
		PenetrationsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "pen",
			Name:      "runs_total",
			Help:      "Total penetration analyses completed",
		}),
		AlertsTotal: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "alert",
			Name:      "total",
			Help:      "Total alerts fired by level and type",
		}, []string{"level", "type"}),
		DBInsertsTotal: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "db",
			Name:      "inserts_total",
			Help:      "Total ClickHouse inserts by table",
		}, []string{"table"}),
		DBInsertErrorsTotal: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "db",
			Name:      "insert_errors_total",
			Help:      "Total ClickHouse insert errors by table",
		}, []string{"table"}),
		MQTTMessagesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "mqtt",
			Name:      "messages_total",
			Help:      "Total MQTT alert messages published",
		}),
		MQTTReconnectsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "ballistics",
			Subsystem: "mqtt",
			Name:      "reconnects_total",
			Help:      "Total MQTT broker reconnections",
		}),
		SimDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "ballistics",
			Subsystem: "sim",
			Name:      "duration_seconds",
			Help:      "Ballistic simulation wall-clock duration",
			Buckets:   prometheus.ExponentialBuckets(1e-6, 2.5, 12),
		}),
		PenDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "ballistics",
			Subsystem: "pen",
			Name:      "duration_seconds",
			Help:      "Penetration analysis wall-clock duration",
			Buckets:   prometheus.ExponentialBuckets(1e-7, 2.5, 10),
		}),
		UDPSizeBytes: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "ballistics",
			Subsystem: "udp",
			Name:      "packet_size_bytes",
			Help:      "UDP sensor packet size distribution",
			Buckets:   prometheus.ExponentialBuckets(64, 1.5, 10),
		}),
		ImpactVelocity: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "ballistics",
			Subsystem: "sim",
			Name:      "impact_velocity_ms",
			Help:      "Impact velocity distribution",
			Buckets:   prometheus.LinearBuckets(20, 20, 9),
		}),
		PenetrationDepth: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "ballistics",
			Subsystem: "pen",
			Name:      "depth_mm",
			Help:      "Penetration depth distribution in mm",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10),
		}),
		ActiveSimulations: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Subsystem: "sim",
			Name:      "active",
			Help:      "Currently running simulations",
		}),
		ActivePenetrations: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Subsystem: "pen",
			Name:      "active",
			Help:      "Currently running penetration analyses",
		}),
		PendingSensorCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Subsystem: "queue",
			Name:      "sensor_pending",
			Help:      "Pending sensor items in input queue",
		}),
		PendingSimCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Subsystem: "queue",
			Name:      "sim_pending",
			Help:      "Pending simulation jobs in queue",
		}),
		PendingPenCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Subsystem: "queue",
			Name:      "pen_pending",
			Help:      "Pending penetration jobs in queue",
		}),
		UpGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Name:      "up",
			Help:      "Service uptime gauge, 1 when alive",
		}),
		StartTime: time.Now(),
	}

	prometheus.MustRegister(
		m.SensorPacketsTotal,
		m.SensorPacketsInvalid,
		m.SimulationsTotal,
		m.PenetrationsTotal,
		&m.AlertsTotal,
		&m.DBInsertsTotal,
		&m.DBInsertErrorsTotal,
		m.MQTTMessagesTotal,
		m.MQTTReconnectsTotal,
		m.SimDurationSeconds,
		m.PenDurationSeconds,
		m.UDPSizeBytes,
		m.ImpactVelocity,
		m.PenetrationDepth,
		m.ActiveSimulations,
		m.ActivePenetrations,
		m.PendingSensorCount,
		m.PendingSimCount,
		m.PendingPenCount,
		m.UpGauge,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ballistics",
			Name:      "start_time_seconds",
			Help:      "Unix timestamp of service start",
		}, func() float64 { return float64(m.StartTime.Unix()) }),
	)
	m.UpGauge.Set(1)
	return m
}

func (m *Metrics) NextSensorSeq() uint64 { return atomic.AddUint64(&m.sensorSeq, 1) - 1 }
func (m *Metrics) NextSimSeq() uint64    { return atomic.AddUint64(&m.simSeq, 1) - 1 }
func (m *Metrics) NextPenSeq() uint64    { return atomic.AddUint64(&m.penSeq, 1) - 1 }
func (m *Metrics) NextAlertSeq() uint64  { return atomic.AddUint64(&m.alertSeq, 1) - 1 }

func (m *Metrics) StartMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("metrics/pprof server: " + err.Error())
		}
	}()
	return srv
}
