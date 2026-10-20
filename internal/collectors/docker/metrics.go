package docker

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

// ContainerMetrics holds all computed metrics for one container
type ContainerMetrics struct {
	Timestamp time.Time

	CPUPercent float64
	MemUsageMB float64
	MemLimitMB float64
	MemPercent float64

	NetInMB  float64
	NetOutMB float64

	BlockReadMB  float64
	BlockWriteMB float64

	PIDs uint32

	// for error display
	Err error
}

// FetchMetrics gets a single stats snapshot for a container
// Docker stats are a stream — we read one entry and close
func FetchMetrics(ctx context.Context, cli *dockerclient.Client, containerID string) ContainerMetrics {
	resp, err := cli.ContainerStats(ctx, containerID, false) // false = not streaming
	if err != nil {
		return ContainerMetrics{Err: err}
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		if err != io.EOF {
			return ContainerMetrics{Err: err}
		}
	}

	return computeMetrics(stats)
}

func computeMetrics(s container.StatsResponse) ContainerMetrics {
	m := ContainerMetrics{Timestamp: time.Now()}

	// ── CPU % ─────────────────────────────────────────────────────────
	// Docker gives raw CPU ticks — need delta between two readings
	// With a single reading we use the precpu stats delta
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) -
		float64(s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage) -
		float64(s.PreCPUStats.SystemUsage)
	numCPUs := float64(s.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		numCPUs = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0 && cpuDelta > 0 {
		m.CPUPercent = (cpuDelta / systemDelta) * numCPUs * 100.0
	}

	// ── Memory ────────────────────────────────────────────────────────
	// subtract cache from usage for a more accurate reading
	cache := s.MemoryStats.Stats["cache"]
	usedBytes := s.MemoryStats.Usage - cache
	m.MemUsageMB = float64(usedBytes) / 1024 / 1024
	m.MemLimitMB = float64(s.MemoryStats.Limit) / 1024 / 1024
	if s.MemoryStats.Limit > 0 {
		m.MemPercent = float64(usedBytes) / float64(s.MemoryStats.Limit) * 100
	}

	// ── Network ───────────────────────────────────────────────────────
	for _, net := range s.Networks {
		m.NetInMB += float64(net.RxBytes) / 1024 / 1024
		m.NetOutMB += float64(net.TxBytes) / 1024 / 1024
	}

	// ── Block I/O ─────────────────────────────────────────────────────
	for _, bio := range s.BlkioStats.IoServiceBytesRecursive {
		switch bio.Op {
		case "Read":
			m.BlockReadMB = float64(bio.Value) / 1024 / 1024
		case "Write":
			m.BlockWriteMB = float64(bio.Value) / 1024 / 1024
		}
	}

	// ── PIDs ──────────────────────────────────────────────────────────
	m.PIDs = uint32(s.PidsStats.Current)

	return m
}
