package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/config"
)

// Список gauge-метрик из runtime
var runtimeGaugeMetrics = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
	"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
	"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse",
	"MSpanSys", "Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys",
	"PauseTotalNs", "StackInuse", "StackSys", "Sys", "TotalAlloc",
}

// MetricValue — структура для хранения значения метрики
type MetricValue struct {
	Type    string
	Gauge   float64
	Counter int64
}

// Agent представляет агент сбора метрик
type Agent struct {
	config  *config.AgentConfig
	metrics map[string]*MetricValue
	client  *http.Client
}

// New создает новый агент с конфигурацией
func New(cfg *config.AgentConfig) *Agent {
	metrics := make(map[string]*MetricValue)

	// Инициализируем все gauge-метрики из runtime
	for _, name := range runtimeGaugeMetrics {
		metrics[name] = &MetricValue{Type: "gauge"}
	}

	// Добавляем специальные метрики
	metrics["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metrics["RandomValue"] = &MetricValue{Type: "gauge", Gauge: rand.Float64()}

	return &Agent{
		config:  cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Run запускает основной цикл агента
func (a *Agent) Run() {
	pollTicker := time.NewTicker(a.config.PollInterval)
	reportTicker := time.NewTicker(a.config.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	log.Println("Agent started")

	for {
		select {
		case <-pollTicker.C:
			a.collectMetrics()

		case <-reportTicker.C:
			log.Println("Sending metrics to server...")
			a.sendMetrics()
		}
	}
}

// collectMetrics собирает метрики из runtime
func (a *Agent) collectMetrics() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Обновляем счётчик опросов
	a.metrics["PollCount"].Counter++

	// Обновляем случайное значение
	a.metrics["RandomValue"].Gauge = rand.Float64()

	// Обновляем все runtime-метрики
	a.updateRuntimeMetrics(&ms)
}

// updateRuntimeMetrics обновляет все метрики из runtime.MemStats
func (a *Agent) updateRuntimeMetrics(ms *runtime.MemStats) {
	a.metrics["Alloc"].Gauge = float64(ms.Alloc)
	a.metrics["BuckHashSys"].Gauge = float64(ms.BuckHashSys)
	a.metrics["Frees"].Gauge = float64(ms.Frees)
	a.metrics["GCCPUFraction"].Gauge = ms.GCCPUFraction
	a.metrics["GCSys"].Gauge = float64(ms.GCSys)
	a.metrics["HeapAlloc"].Gauge = float64(ms.HeapAlloc)
	a.metrics["HeapIdle"].Gauge = float64(ms.HeapIdle)
	a.metrics["HeapInuse"].Gauge = float64(ms.HeapInuse)
	a.metrics["HeapObjects"].Gauge = float64(ms.HeapObjects)
	a.metrics["HeapReleased"].Gauge = float64(ms.HeapReleased)
	a.metrics["HeapSys"].Gauge = float64(ms.HeapSys)
	a.metrics["LastGC"].Gauge = float64(ms.LastGC)
	a.metrics["Lookups"].Gauge = float64(ms.Lookups)
	a.metrics["MCacheInuse"].Gauge = float64(ms.MCacheInuse)
	a.metrics["MCacheSys"].Gauge = float64(ms.MCacheSys)
	a.metrics["MSpanInuse"].Gauge = float64(ms.MSpanInuse)
	a.metrics["MSpanSys"].Gauge = float64(ms.MSpanSys)
	a.metrics["Mallocs"].Gauge = float64(ms.Mallocs)
	a.metrics["NextGC"].Gauge = float64(ms.NextGC)
	a.metrics["NumForcedGC"].Gauge = float64(ms.NumForcedGC)
	a.metrics["NumGC"].Gauge = float64(ms.NumGC)
	a.metrics["OtherSys"].Gauge = float64(ms.OtherSys)
	a.metrics["PauseTotalNs"].Gauge = float64(ms.PauseTotalNs)
	a.metrics["StackInuse"].Gauge = float64(ms.StackInuse)
	a.metrics["StackSys"].Gauge = float64(ms.StackSys)
	a.metrics["Sys"].Gauge = float64(ms.Sys)
	a.metrics["TotalAlloc"].Gauge = float64(ms.TotalAlloc)
}

// sendMetrics отправляет все метрики на сервер
func (a *Agent) sendMetrics() {
	successCount := 0
	errorCount := 0

	for name, metric := range a.metrics {
		if err := a.sendMetric(name, metric); err != nil {
			log.Printf("Failed to send metric %s: %v", name, err)
			errorCount++
		} else {
			successCount++
		}
	}

	log.Printf("Metrics sent: %d success, %d errors", successCount, errorCount)
}

// sendMetric отправляет одну метрику на сервер
func (a *Agent) sendMetric(name string, metric *MetricValue) error {
	url := a.config.GetServerURL() + "/update"

	var jsonMetric struct {
		ID    string   `json:"id"`
		MType string   `json:"type"`
		Delta *int64   `json:"delta,omitempty"`
		Value *float64 `json:"value,omitempty"`
	}
	jsonMetric.ID = name
	jsonMetric.MType = metric.Type
	if metric.Type == "counter" {
		jsonMetric.Delta = &metric.Counter
	} else if metric.Type == "gauge" {
		jsonMetric.Value = &metric.Gauge
	}

	buf, err := json.Marshal(jsonMetric)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "metrics-agent/1.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	return nil
}

// buildMetricURL строит URL для отправки метрики
func (a *Agent) buildMetricURL(name string, metric *MetricValue) string {
	switch metric.Type {
	case "gauge":
		// Используем 'g' для компактного формата без лишних нулей
		valueStr := strconv.FormatFloat(metric.Gauge, 'g', -1, 64)
		return fmt.Sprintf("%s/update/gauge/%s/%s", a.config.GetServerURL(), name, valueStr)
	case "counter":
		return fmt.Sprintf("%s/update/counter/%s/%d", a.config.GetServerURL(), name, metric.Counter)
	default:
		return ""
	}
}

// GetMetrics возвращает копию текущих метрик (для тестирования)
func (a *Agent) GetMetrics() map[string]*MetricValue {
	result := make(map[string]*MetricValue)
	for name, metric := range a.metrics {
		result[name] = &MetricValue{
			Type:    metric.Type,
			Gauge:   metric.Gauge,
			Counter: metric.Counter,
		}
	}
	return result
}

// GetMetric возвращает значение конкретной метрики (для тестирования)
func (a *Agent) GetMetric(name string) (*MetricValue, bool) {
	metric, exists := a.metrics[name]
	if !exists {
		return nil, false
	}
	return &MetricValue{
		Type:    metric.Type,
		Gauge:   metric.Gauge,
		Counter: metric.Counter,
	}, true
}
