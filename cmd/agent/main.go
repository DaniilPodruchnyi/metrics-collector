package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
)

var serverAddr = "http://localhost:8080"

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

func main() {
	// Создаём map с указателями на MetricValue
	metrics := make(map[string]*MetricValue)

	// Инициализируем все gauge-метрики из runtime
	for _, name := range runtimeGaugeMetrics {
		metrics[name] = &MetricValue{Type: "gauge"}
	}

	// Добавляем специальные метрики
	metrics["PollCount"] = &MetricValue{Type: "counter", Counter: 0}
	metrics["RandomValue"] = &MetricValue{Type: "gauge", Gauge: rand.Float64()}

	// Тикеры
	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	log.Println("Agent started. Polling - 2s, reporting - 10s.")

	for {
		select {
		case <-pollTicker.C:
			// Сбор данных из runtime
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)

			// Обновляем счётчик опросов
			metrics["PollCount"].Counter++

			// Обновляем случайное значение
			metrics["RandomValue"].Gauge = rand.Float64()

			// Обновляем все runtime-метрики
			metrics["Alloc"].Gauge = float64(ms.Alloc)
			metrics["BuckHashSys"].Gauge = float64(ms.BuckHashSys)
			metrics["Frees"].Gauge = float64(ms.Frees)
			metrics["GCCPUFraction"].Gauge = ms.GCCPUFraction
			metrics["GCSys"].Gauge = float64(ms.GCSys)
			metrics["HeapAlloc"].Gauge = float64(ms.HeapAlloc)
			metrics["HeapIdle"].Gauge = float64(ms.HeapIdle)
			metrics["HeapInuse"].Gauge = float64(ms.HeapInuse)
			metrics["HeapObjects"].Gauge = float64(ms.HeapObjects)
			metrics["HeapReleased"].Gauge = float64(ms.HeapReleased)
			metrics["HeapSys"].Gauge = float64(ms.HeapSys)
			metrics["LastGC"].Gauge = float64(ms.LastGC)
			metrics["Lookups"].Gauge = float64(ms.Lookups)
			metrics["MCacheInuse"].Gauge = float64(ms.MCacheInuse)
			metrics["MCacheSys"].Gauge = float64(ms.MCacheSys)
			metrics["MSpanInuse"].Gauge = float64(ms.MSpanInuse)
			metrics["MSpanSys"].Gauge = float64(ms.MSpanSys)
			metrics["Mallocs"].Gauge = float64(ms.Mallocs)
			metrics["NextGC"].Gauge = float64(ms.NextGC)
			metrics["NumForcedGC"].Gauge = float64(ms.NumForcedGC)
			metrics["NumGC"].Gauge = float64(ms.NumGC)
			metrics["OtherSys"].Gauge = float64(ms.OtherSys)
			metrics["PauseTotalNs"].Gauge = float64(ms.PauseTotalNs)
			metrics["StackInuse"].Gauge = float64(ms.StackInuse)
			metrics["StackSys"].Gauge = float64(ms.StackSys)
			metrics["Sys"].Gauge = float64(ms.Sys)
			metrics["TotalAlloc"].Gauge = float64(ms.TotalAlloc)

		case <-reportTicker.C:
			log.Println("Sending metrics to server...")
			for name, m := range metrics {
				if err := sendMetric(name, m); err != nil {
					log.Printf("Failed to send metric %s: %v", name, err)
				}
			}
		}
	}
}

// sendMetric отправляет одну метрику на сервер по HTTP
func sendMetric(name string, m *MetricValue) error {
	var url string
	switch m.Type {
	case "gauge":
		// Используем 'g' для компактного формата без лишних нулей
		valueStr := strconv.FormatFloat(m.Gauge, 'g', -1, 64)
		url = fmt.Sprintf("%s/update/gauge/%s/%s", serverAddr, name, valueStr)
	case "counter":
		url = fmt.Sprintf("%s/update/counter/%s/%d", serverAddr, name, m.Counter)
	default:
		return fmt.Errorf("unknown metric type: %s", m.Type)
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	return nil
}
