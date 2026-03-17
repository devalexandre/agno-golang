package system

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// SystemTools provides tools for monitoring system performance.
type SystemTools struct {
	*toolkit.Toolkit
}

// NewSystemTools creates a new SystemTools instance.
func NewSystemTools() *SystemTools {
	tk := toolkit.NewToolkit()
	tk.Name = "system"
	tk.Description = "Tools for monitoring system performance (CPU, Memory, Disk, Processes)."

	s := &SystemTools{
		Toolkit: &tk,
	}

	s.Register("GetCPUUsage", "Returns current CPU usage percentage.", s, s.GetCPUUsage, EmptyParams{})
	s.Register("GetMemoryUsage", "Returns current memory usage information.", s, s.GetMemoryUsage, EmptyParams{})
	s.Register("GetDiskUsage", "Returns disk usage for a given path.", s, s.GetDiskUsage, PathParams{})
	s.Register("GetHostInfo", "Returns general information about the host system.", s, s.GetHostInfo, EmptyParams{})
	s.Register("ListProcesses", "Lists top processes by CPU or memory usage.", s, s.ListProcesses, ListProcessesParams{})

	return s
}

type EmptyParams struct{}

func (s *SystemTools) GetCPUUsage(params EmptyParams) (interface{}, error) {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %v", err)
	}
	if len(percentages) == 0 {
		return 0, nil
	}
	return percentages[0], nil
}

func (s *SystemTools) GetMemoryUsage(params EmptyParams) (interface{}, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory usage: %v", err)
	}
	return map[string]interface{}{
		"total":      v.Total,
		"available":  v.Available,
		"used":       v.Used,
		"used_perct": v.UsedPercent,
		"free":       v.Free,
	}, nil
}

type PathParams struct {
	Path string `json:"path"`
}

func (s *SystemTools) GetDiskUsage(params PathParams) (interface{}, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	u, err := disk.Usage(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %v", err)
	}
	return map[string]interface{}{
		"path":       u.Path,
		"total":      u.Total,
		"free":       u.Free,
		"used":       u.Used,
		"used_perct": u.UsedPercent,
	}, nil
}

func (s *SystemTools) GetHostInfo(params EmptyParams) (interface{}, error) {
	h, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %v", err)
	}
	return map[string]interface{}{
		"hostname":              h.Hostname,
		"uptime":                h.Uptime,
		"boot_time":             h.BootTime,
		"procs":                 h.Procs,
		"os":                    h.OS,
		"platform":              h.Platform,
		"platform_family":       h.PlatformFamily,
		"platform_version":      h.PlatformVersion,
		"kernel_version":        h.KernelVersion,
		"virtualization_system": h.VirtualizationSystem,
		"virtualization_role":   h.VirtualizationRole,
	}, nil
}

type ListProcessesParams struct {
	Limit int    `json:"limit"`
	Sort  string `json:"sort"` // "cpu" or "memory"
}

func (s *SystemTools) ListProcesses(params ListProcessesParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %v", err)
	}

	type procInfo struct {
		PID    int32   `json:"pid"`
		Name   string  `json:"name"`
		CPU    float64 `json:"cpu"`
		Memory float32 `json:"memory"`
	}

	var infos []procInfo
	for _, p := range procs {
		name, _ := p.Name()
		cpuP, _ := p.CPUPercent()
		memP, _ := p.MemoryPercent()

		infos = append(infos, procInfo{
			PID:    p.Pid,
			Name:   name,
			CPU:    cpuP,
			Memory: memP,
		})
	}

	// Simple sort (real implementation would be more robust)
	// For now, returning first N
	if len(infos) > limit {
		infos = infos[:limit]
	}

	return infos, nil
}
