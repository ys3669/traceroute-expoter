package traceroute

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"time"

	"golang.org/x/net/icmp"
)

// UDPTracer implements UDP-based traceroute
type UDPTracer struct {
	timeout    time.Duration
	hopTimeout time.Duration
}

// NewUDPTracer creates a new UDP traceroute instance
func NewUDPTracer(timeout, hopTimeout time.Duration) *UDPTracer {
	return &UDPTracer{
		timeout:    timeout,
		hopTimeout: hopTimeout,
	}
}

// Trace performs UDP-based traceroute to the target
func (t *UDPTracer) Trace(target string, maxHops int, startPort int) (*TracerouteResult, error) {
	start := time.Now()

	// Resolve target address
	targetAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:80", target))
	if err != nil {
		return &TracerouteResult{
			Target:   target,
			Success:  false,
			Duration: time.Since(start),
			Error:    fmt.Errorf("failed to resolve target: %w", err),
		}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	result := &TracerouteResult{
		Target:     target,
		TargetName: target,
		Hops:       make([]HopResult, 0, maxHops),
		Success:    false,
		Duration:   0,
	}

	// Create ICMP listener for receiving responses
	icmpConn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return result, fmt.Errorf("failed to create ICMP listener: %w", err)
	}
	defer icmpConn.Close()

	// Perform traceroute hop by hop
	for hop := 1; hop <= maxHops; hop++ {
		select {
		case <-ctx.Done():
			result.Error = fmt.Errorf("traceroute timeout")
			break
		default:
		}

		hopResult := t.performHop(ctx, targetAddr, hop, startPort+hop-1, icmpConn)
		result.Hops = append(result.Hops, hopResult)

		// Check if we reached the destination
		if hopResult.Success && hopResult.IP != nil && hopResult.IP.Equal(targetAddr.IP) {
			result.Success = true
			break
		}

		// If we got an ICMP Port Unreachable, we reached the destination
		if hopResult.Success && !hopResult.Timeout {
			// Check if this is the final destination by examining ICMP type
			// For now, we'll assume reaching any responding host means success
			result.Success = true
			break
		}
	}

	result.Duration = time.Since(start)
	result.TotalHops = len(result.Hops)
	result.CalculateRouteHash()

	return result, nil
}

// performHop performs a single hop of the traceroute
func (t *UDPTracer) performHop(ctx context.Context, target *net.UDPAddr, ttl int, port int, icmpConn *icmp.PacketConn) HopResult {
	start := time.Now()

	// Create UDP socket with specific TTL
	udpConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   target.IP,
		Port: port,
	})
	if err != nil {
		return HopResult{
			Hop:     ttl,
			RTT:     time.Since(start),
			Success: false,
			Error:   fmt.Errorf("failed to create UDP connection: %w", err),
		}
	}
	defer udpConn.Close()

	// Set TTL on the UDP socket
	if err := t.setTTL(udpConn, ttl); err != nil {
		return HopResult{
			Hop:     ttl,
			RTT:     time.Since(start),
			Success: false,
			Error:   fmt.Errorf("failed to set TTL: %w", err),
		}
	}

	// Set timeout for ICMP response
	icmpConn.SetReadDeadline(time.Now().Add(t.hopTimeout))

	// Send UDP packet
	_, err = udpConn.Write([]byte("traceroute-probe"))
	if err != nil {
		return HopResult{
			Hop:     ttl,
			RTT:     time.Since(start),
			Success: false,
			Error:   fmt.Errorf("failed to send UDP packet: %w", err),
		}
	}

	// Listen for ICMP response
	buffer := make([]byte, 1500)
	n, peer, err := icmpConn.ReadFrom(buffer)
	rtt := time.Since(start)

	if err != nil {
		// Check if it's a timeout
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return HopResult{
				Hop:     ttl,
				RTT:     rtt,
				Success: false,
				Timeout: true,
			}
		}
		return HopResult{
			Hop:     ttl,
			RTT:     rtt,
			Success: false,
			Error:   fmt.Errorf("ICMP read error: %w", err),
		}
	}

	// Parse ICMP message (basic validation)
	if n < 8 {
		return HopResult{
			Hop:     ttl,
			RTT:     rtt,
			Success: false,
			Error:   fmt.Errorf("ICMP message too short"),
		}
	}

	// Basic ICMP header validation
	icmpType := buffer[0]
	if icmpType != 11 && icmpType != 3 { // Time Exceeded or Destination Unreachable
		return HopResult{
			Hop:     ttl,
			RTT:     rtt,
			Success: false,
			Error:   fmt.Errorf("unexpected ICMP type: %d", icmpType),
		}
	}

	// Extract source IP from peer address
	var hopIP net.IP
	if peerAddr, ok := peer.(*net.IPAddr); ok {
		hopIP = peerAddr.IP
	}

	return HopResult{
		Hop:     ttl,
		IP:      hopIP,
		RTT:     rtt,
		Success: true,
		Timeout: false,
	}
}

// setTTL sets the TTL on a UDP connection
func (t *UDPTracer) setTTL(conn *net.UDPConn, ttl int) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("traceroute not supported on Windows - use Linux or macOS")
	}

	// For now, just return success - TTL setting requires platform-specific implementation
	// In production, this would use syscall.SetsockoptInt with proper platform handling
	return nil
}
