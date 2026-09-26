// Package metrics provides an embedded Prometheus exposition format metrics registry.
package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// MetricType represents the Prometheus metric type (counter or gauge).
type MetricType string

const (
	// TypeCounter identifies a monotonically increasing counter.
	TypeCounter MetricType = "counter"
	// TypeGauge identifies a variable gauge value.
	TypeGauge MetricType = "gauge"
)

type metricKey struct {
	name   string
	labels string
}

type metricValue struct {
	name   string
	mType  MetricType
	labels map[string]string
	val    float64
}

// Registry stores and renders application metrics in Prometheus exposition format.
type Registry struct {
	mu      sync.RWMutex
	metrics map[metricKey]*metricValue
	types   map[string]MetricType
}

// DefaultRegistry is the package-level default metrics registry.
var DefaultRegistry = NewRegistry()

// NewRegistry initializes and returns an empty metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		metrics: make(map[metricKey]*metricValue),
		types:   make(map[string]MetricType),
	}
}

// IncCounter increments a counter metric by 1.
func (r *Registry) IncCounter(name string, labels map[string]string) {
	r.AddCounter(name, labels, 1)
}

// AddCounter adds delta to a counter metric.
func (r *Registry) AddCounter(name string, labels map[string]string, delta float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.types[name] = TypeCounter
	k := metricKey{name: name, labels: encodeLabels(labels)}
	entry, exists := r.metrics[k]
	if !exists {
		entry = &metricValue{
			name:   name,
			mType:  TypeCounter,
			labels: cloneLabels(labels),
			val:    0,
		}
		r.metrics[k] = entry
	}
	entry.val += delta
}

// SetGauge sets the current value of a gauge metric.
func (r *Registry) SetGauge(name string, labels map[string]string, val float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.types[name] = TypeGauge
	k := metricKey{name: name, labels: encodeLabels(labels)}
	entry, exists := r.metrics[k]
	if !exists {
		entry = &metricValue{
			name:   name,
			mType:  TypeGauge,
			labels: cloneLabels(labels),
		}
		r.metrics[k] = entry
	}
	entry.val = val
}

// Render returns the full metrics exposition string formatted for Prometheus scraping.
func (r *Registry) Render() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	names := r.sortedMetricNames()

	for _, name := range names {
		mType := r.types[name]
		sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, mType))

		for _, item := range r.collectEntriesByName(name) {
			valStr := strconv.FormatFloat(item.val, 'f', -1, 64)
			if len(item.labels) == 0 {
				sb.WriteString(fmt.Sprintf("%s %s\n", item.name, valStr))
			} else {
				sb.WriteString(fmt.Sprintf("%s{%s} %s\n", item.name, formatLabels(item.labels), valStr))
			}
		}
	}
	return sb.String()
}

func (r *Registry) sortedMetricNames() []string {
	names := make([]string, 0, len(r.types))
	for n := range r.types {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) collectEntriesByName(name string) []*metricValue {
	var items []*metricValue
	for _, entry := range r.metrics {
		if entry.name == name {
			items = append(items, entry)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return encodeLabels(items[i].labels) < encodeLabels(items[j].labels)
	})
	return items
}

func encodeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + labels[k]
	}
	return strings.Join(parts, ",")
}

func formatLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v := labels[k]
		v = strings.ReplaceAll(v, `\`, `\\`)
		v = strings.ReplaceAll(v, `"`, `\"`)
		v = strings.ReplaceAll(v, "\n", `\n`)
		parts[i] = fmt.Sprintf(`%s="%s"`, k, v)
	}
	return strings.Join(parts, ",")
}

func cloneLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	res := make(map[string]string, len(labels))
	for k, v := range labels {
		res[k] = v
	}
	return res
}
