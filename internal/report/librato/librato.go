package librato

import (
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ContaAzul/hystrix-to-librato/internal/models"
	librato "github.com/rcrowley/go-librato"
)

// New report type
func New(user, token string, metrics []string, interval time.Duration) *Librato {
	return &Librato{
		user:     user,
		token:    token,
		reports:  make(map[string]time.Time),
		lock:     sync.RWMutex{},
		metrics:  metrics,
		interval: interval,
	}
}

// Librato type
type Librato struct {
	user     string
	token    string
	reports  map[string]time.Time
	lock     sync.RWMutex
	metrics  []string
	interval time.Duration
}

// Report the given data to librato for the given cluster
func (r *Librato) Report(data models.Data, cluster string) {
	source1 := cluster + "." + data.Group
	source2 := cluster + "." + data.Group + "." + data.Name
	if r.shouldReport(source1) {
		log.Println("Report", source1)
		r.circuitOpen(data, source1)
	}
	if r.shouldReport(source2) {
		log.Println("Report", source2)
		r.latencies(data, source2)
	}
}

func (r *Librato) latencies(data models.Data, source string) {
	m := librato.NewSimpleMetrics(r.user, r.token, source)
	defer m.Wait()
	defer m.Close()

	latencies := reflect.ValueOf(data.LatencieTotals)
	for _, metric := range r.metrics {
		if metric == "mean" {
			m.NewCounter("hystrix.latency.mean") <- data.MeanLatency
			continue
		}
		name := strings.Replace(
			strings.Replace(metric, ".", "", -1),
			"th", "", -1,
		)
		latency := reflect.Indirect(latencies).FieldByName("L" + name)
		m.NewCounter("hystrix.latency." + metric) <- latency.Int()
	}
}

func (r *Librato) circuitOpen(data models.Data, source string) {
	m := librato.NewSimpleMetrics(r.user, r.token, source)
	defer m.Wait()
	defer m.Close()

	c := m.NewCounter("hystrix.circuit.open")
	if isOpen(data.Open) {
		c <- 1
	} else {
		c <- 0
	}
}

func (r *Librato) shouldReport(source string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	val, ok := r.reports[source]
	if ok && time.Since(val).Seconds() < r.interval.Seconds() {
		return false
	}
	r.reports[source] = time.Now()
	return true
}

func isOpen(data interface{}) bool {
	if b, ok := data.(bool); ok {
		return b
	}
	return true
}
