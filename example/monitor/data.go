package main

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/widgets"
)

// historyLength keeps one sample beyond the visible window so the oldest
// point can slide out while a new one slides in.
const historyLength = 61

const (
	cpuCores    = 8
	memoryTotal = 16.0 // GB
)

// readings are the headline values. They are interpolated for display.
type readings struct {
	CPU, Memory, Up, Down float32 // percent, GB, MB/s, MB/s
}

func lerpReadings(from, to readings, t float64) readings {
	f := func(a, b float32) float32 { return a + (b-a)*float32(t) }
	return readings{f(from.CPU, to.CPU), f(from.Memory, to.Memory), f(from.Up, to.Up), f(from.Down, to.Down)}
}

type process struct {
	PID         int
	Name        string
	CPU, base   float32 // percent of one core
	Memory, mem float32 // MB; mem is the resting value
}

// processModel adapts the process list to TableView with PIDs as stable IDs.
type processModel struct{ *gui.SliceListModel[process] }

func (m processModel) RowID(i int) string { return fmt.Sprint(m.ItemAt(i).PID) }

// simulator produces plausible, reproducible fake system activity. It never
// queries the real system.
type simulator struct {
	rng                *rand.Rand
	current            readings
	cpu, up, down      []float32
	processes          []process
	cpuTarget, netBase float32
}

func newSimulator() *simulator {
	s := &simulator{rng: rand.New(rand.NewPCG(7, 42)), cpuTarget: 32, netBase: 1.4}
	s.current = readings{CPU: 32, Memory: 9.4, Up: .3, Down: 1.2}
	seed := []struct {
		name     string
		cpu, mem float32
	}{
		{"goui-monitor", 1.2, 48}, {"Xorg", 4.8, 212}, {"chrome", 11.5, 1480}, {"code", 6.2, 920},
		{"gopls", 3.1, 640}, {"postgres", 2.4, 386}, {"redis-server", .8, 74}, {"nginx", .4, 22},
		{"dockerd", 1.6, 158}, {"node", 5.4, 512}, {"pipewire", 1.1, 36}, {"firefox", 7.9, 1120},
		{"containerd", .9, 96}, {"prometheus", 2.2, 284}, {"grafana", 1.4, 172}, {"systemd", .2, 18},
		{"slack", 2.8, 610}, {"go build", 18, 420},
	}
	for i, p := range seed {
		s.processes = append(s.processes, process{PID: 1200 + i*137%3100, Name: p.name, CPU: p.cpu, base: p.cpu, Memory: p.mem, mem: p.mem})
	}
	for range historyLength {
		s.step()
	}
	return s
}

// step advances one second of activity and appends to every history.
func (s *simulator) step() {
	r := s.rng
	// CPU drifts towards a target that changes occasionally, with rare bursts.
	if r.Float32() < .12 {
		s.cpuTarget = 18 + r.Float32()*40
	}
	cpu := s.current.CPU + (s.cpuTarget-s.current.CPU)*.35 + (r.Float32()-.5)*9
	if r.Float32() < .05 {
		cpu += 25 + r.Float32()*25
	}
	s.current.CPU = clamp(cpu, 3, 98)
	s.current.Memory = clamp(s.current.Memory+(r.Float32()-.5)*.18+(9.4-s.current.Memory)*.05, 6, memoryTotal-.5)
	if r.Float32() < .1 {
		s.netBase = .3 + r.Float32()*4
	}
	s.current.Down = clamp(s.netBase*(.6+r.Float32()*.8), .02, 12)
	s.current.Up = clamp(s.current.Down*(.12+r.Float32()*.25), .01, 4)
	s.cpu = pushSample(s.cpu, s.current.CPU)
	s.up = pushSample(s.up, s.current.Up)
	s.down = pushSample(s.down, s.current.Down)
	load := s.current.CPU / 32
	for i := range s.processes {
		p := &s.processes[i]
		p.CPU = clamp(p.base*load*(.7+r.Float32()*.6), 0, 99)
		p.Memory = max(4, p.mem*(.96+r.Float32()*.08))
	}
}

func pushSample(history []float32, v float32) []float32 {
	if len(history) < historyLength {
		return append(history, v)
	}
	copy(history, history[1:])
	history[len(history)-1] = v
	return history
}

// sortedProcesses returns an independent, sorted copy for the table model.
func (s *simulator) sortedProcesses(column string, order widgets.SortOrder) []process {
	items := slices.Clone(s.processes)
	slices.SortStableFunc(items, func(a, b process) int {
		var n int
		switch column {
		case "cpu":
			n = cmp.Compare(a.CPU, b.CPU)
		case "memory":
			n = cmp.Compare(a.Memory, b.Memory)
		case "name":
			n = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		if order == widgets.SortNone || n == 0 {
			return cmp.Compare(a.PID, b.PID)
		}
		if order == widgets.SortDescending {
			return -n
		}
		return n
	})
	return items
}

func clamp(v, lo, hi float32) float32 { return min(hi, max(lo, v)) }

func formatMemory(mb float32) string {
	if mb < 1024 {
		return fmt.Sprintf("%.0f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}

func formatRate(mbps float32) string {
	if mbps < 1 {
		return fmt.Sprintf("%.0f KB/s", mbps*1024)
	}
	return fmt.Sprintf("%.1f MB/s", mbps)
}
