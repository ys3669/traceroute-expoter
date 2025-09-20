package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ys3669/traceroute-exporter/config"
	"github.com/ys3669/traceroute-exporter/traceroute"
)

var (
	// Traceroute hop response time in seconds
	tracerouteHopRTT = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_hop_rtt_seconds",
			Help: "Response time for each hop in traceroute",
		},
		[]string{"target", "target_name", "hop", "hop_ip"},
	)

	// Traceroute hop success/failure
	tracerouteHopSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_hop_success",
			Help: "Hop success (1 = success, 0 = failure)",
		},
		[]string{"target", "target_name", "hop", "hop_ip"},
	)

	// Traceroute hop timeout
	tracerouteHopTimeout = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_hop_timeout",
			Help: "Hop timeout (1 = timeout, 0 = no timeout)",
		},
		[]string{"target", "target_name", "hop"},
	)

	// Total hops in traceroute
	tracerouteTotalHops = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_total_hops",
			Help: "Total number of hops in traceroute",
		},
		[]string{"target", "target_name"},
	)

	// Route hash for change detection
	tracerouteRouteHash = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_route_hash",
			Help: "Hash of the route for change detection",
		},
		[]string{"target", "target_name"},
	)

	// Traceroute execution time
	tracerouteExecutionSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "traceroute_execution_seconds",
			Help: "Time taken to complete traceroute",
		},
		[]string{"target", "target_name"},
	)

	// Total traceroute executions
	tracerouteExecutionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "traceroute_execution_total",
			Help: "Total number of traceroute executions",
		},
		[]string{"target", "target_name", "status"},
	)
)

var (
	// Custom registry without Go runtime metrics
	customRegistry = prometheus.NewRegistry()
)

func init() {
	// Register metrics with custom registry
	customRegistry.MustRegister(tracerouteHopRTT)
	customRegistry.MustRegister(tracerouteHopSuccess)
	customRegistry.MustRegister(tracerouteHopTimeout)
	customRegistry.MustRegister(tracerouteTotalHops)
	customRegistry.MustRegister(tracerouteRouteHash)
	customRegistry.MustRegister(tracerouteExecutionSeconds)
	customRegistry.MustRegister(tracerouteExecutionTotal)
}

// updateMetrics updates Prometheus metrics based on traceroute result
func updateMetrics(result *traceroute.TracerouteResult) {
	baseLabels := prometheus.Labels{
		"target":      result.Target,
		"target_name": result.TargetName,
	}

	// Update execution metrics
	tracerouteExecutionSeconds.With(baseLabels).Set(result.Duration.Seconds())
	tracerouteTotalHops.With(baseLabels).Set(float64(result.TotalHops))
	tracerouteRouteHash.With(baseLabels).Set(float64(result.RouteHash))

	status := "failure"
	if result.Success {
		status = "success"
	}
	tracerouteExecutionTotal.With(prometheus.Labels{
		"target":      result.Target,
		"target_name": result.TargetName,
		"status":      status,
	}).Inc()

	// Update hop metrics
	for _, hop := range result.Hops {
		hopLabels := prometheus.Labels{
			"target":      result.Target,
			"target_name": result.TargetName,
			"hop":         fmt.Sprintf("%d", hop.Hop),
		}

		if hop.Success && hop.IP != nil {
			hopLabels["hop_ip"] = hop.IP.String()
			tracerouteHopRTT.With(hopLabels).Set(hop.RTT.Seconds())
			tracerouteHopSuccess.With(hopLabels).Set(1)
		} else {
			hopLabels["hop_ip"] = ""
			if hop.Timeout {
				tracerouteHopTimeout.With(prometheus.Labels{
					"target":      result.Target,
					"target_name": result.TargetName,
					"hop":         fmt.Sprintf("%d", hop.Hop),
				}).Set(1)
			}
			tracerouteHopSuccess.With(hopLabels).Set(0)
		}
	}
}

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting traceroute exporter on port %d", cfg.Server.Port)
	log.Printf("Monitoring interval: %v", cfg.Monitoring.Interval)
	log.Printf("Traceroute timeout: %v", cfg.Monitoring.Timeout)
	log.Printf("Hop timeout: %v", cfg.Monitoring.HopTimeout)

	// Create UDP tracer
	tracer := traceroute.NewUDPTracer(cfg.Monitoring.Timeout, cfg.Monitoring.HopTimeout)

	// Start traceroute monitoring
	go func() {
		ticker := time.NewTicker(cfg.Monitoring.Interval)
		defer ticker.Stop()

		for {
			for _, target := range cfg.Targets {
				log.Printf("Tracing route to %s (%s), max_hops=%d", target.Host, target.Name, target.MaxHops)

				result, err := tracer.Trace(target.Host, target.MaxHops, target.StartPort)
				if err != nil {
					log.Printf("Traceroute to %s failed: %v", target.Host, err)
				} else {
					result.TargetName = target.Name
					log.Printf("Traceroute to %s completed: %d hops, %v", target.Host, result.TotalHops, result.Duration)
				}

				// Update metrics regardless of success/failure
				if result != nil {
					updateMetrics(result)
				}
			}
			<-ticker.C
		}
	}()

	// Setup HTTP server with custom registry
	http.Handle("/metrics", promhttp.HandlerFor(customRegistry, promhttp.HandlerOpts{}))

	listenAddr := cfg.GetListenAddress()
	log.Printf("Server starting on %s", listenAddr)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
