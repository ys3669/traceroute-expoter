package traceroute

import (
	"net"
	"time"
)

// HopResult represents the result of a single hop
type HopResult struct {
	Hop     int           `json:"hop"`
	IP      net.IP        `json:"ip,omitempty"`
	RTT     time.Duration `json:"rtt"`
	Success bool          `json:"success"`
	Timeout bool          `json:"timeout"`
	Error   error         `json:"error,omitempty"`
}

// TracerouteResult represents the complete traceroute result
type TracerouteResult struct {
	Target     string        `json:"target"`
	TargetName string        `json:"target_name"`
	Hops       []HopResult   `json:"hops"`
	TotalHops  int           `json:"total_hops"`
	Success    bool          `json:"success"`
	Duration   time.Duration `json:"duration"`
	RouteHash  uint32        `json:"route_hash"`
	Error      error         `json:"error,omitempty"`
}

// IsComplete returns true if the traceroute reached the destination
func (tr *TracerouteResult) IsComplete() bool {
	return tr.Success && len(tr.Hops) > 0
}

// GetSuccessfulHops returns the number of successful hops
func (tr *TracerouteResult) GetSuccessfulHops() int {
	count := 0
	for _, hop := range tr.Hops {
		if hop.Success {
			count++
		}
	}
	return count
}

// CalculateRouteHash calculates a hash of the route for change detection
func (tr *TracerouteResult) CalculateRouteHash() {
	// Simple hash calculation based on hop IPs
	var hash uint32 = 0
	for _, hop := range tr.Hops {
		if hop.Success && hop.IP != nil {
			for _, b := range hop.IP {
				hash = hash*31 + uint32(b)
			}
		}
	}
	tr.RouteHash = hash
}
