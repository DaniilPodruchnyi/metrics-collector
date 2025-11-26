package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository реализует хранение метрик в PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository создает новый PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

// Store сохраняет метрику в БД
func (r *PostgresRepository) Store(metric *model.Metrics) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        INSERT INTO metrics (id, type, delta, value, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (id) 
        DO UPDATE SET 
            type = EXCLUDED.type,
            delta = EXCLUDED.delta,
            value = EXCLUDED.value,
            updated_at = EXCLUDED.updated_at
    `

	_, err := r.pool.Exec(ctx, query,
		metric.ID,
		metric.MType,
		metric.Delta,
		metric.Value,
		time.Now(),
	)

	if err != nil {
		// В продакшене лучше возвращать ошибку или логировать
		fmt.Printf("Failed to store metric %s: %v\n", metric.ID, err)
	}
}

// Get возвращает метрику по имени
func (r *PostgresRepository) Get(name string) (*model.Metrics, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT id, type, delta, value
        FROM metrics
        WHERE id = $1
    `

	var metric model.Metrics
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&metric.ID,
		&metric.MType,
		&metric.Delta,
		&metric.Value,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false
		}
		fmt.Printf("Failed to get metric %s: %v\n", name, err)
		return nil, false
	}

	return &metric, true
}

// GetAll возвращает все метрики
func (r *PostgresRepository) GetAll() map[string]*model.Metrics {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        SELECT id, type, delta, value
        FROM metrics
        ORDER BY id
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		fmt.Printf("Failed to get all metrics: %v\n", err)
		return make(map[string]*model.Metrics)
	}
	defer rows.Close()

	result := make(map[string]*model.Metrics)
	for rows.Next() {
		var metric model.Metrics
		err := rows.Scan(
			&metric.ID,
			&metric.MType,
			&metric.Delta,
			&metric.Value,
		)
		if err != nil {
			fmt.Printf("Failed to scan metric: %v\n", err)
			continue
		}
		result[metric.ID] = &metric
	}

	return result
}

// StoreBatch сохраняет множество метрик в одной транзакции
func (r *PostgresRepository) StoreBatch(metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Начинаем транзакцию
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback если не было Commit

	// Группируем метрики по типу для оптимизации
	counters := make(map[string]*model.Metrics)
	gauges := make(map[string]*model.Metrics)

	for i := range metrics {
		m := &metrics[i]
		if m.MType == model.Counter {
			counters[m.ID] = m
		} else {
			gauges[m.ID] = m
		}
	}

	// Обрабатываем counters (нужно аккумулировать)
	for id, metric := range counters {
		// Получаем существующее значение
		var existingDelta *int64
		query := `SELECT delta FROM metrics WHERE id = $1 AND type = 'counter'`
		err := tx.QueryRow(ctx, query, id).Scan(&existingDelta)

		var newDelta int64
		if err == nil && existingDelta != nil {
			// Аккумулируем
			newDelta = *existingDelta + *metric.Delta
		} else if errors.Is(err, pgx.ErrNoRows) {
			// Новая метрика
			newDelta = *metric.Delta
		} else if err != nil {
			return fmt.Errorf("failed to query counter %s: %w", id, err)
		}

		// Сохраняем
		upsertQuery := `
            INSERT INTO metrics (id, type, delta, value, updated_at)
            VALUES ($1, 'counter', $2, NULL, $3)
            ON CONFLICT (id) 
            DO UPDATE SET delta = EXCLUDED.delta, updated_at = EXCLUDED.updated_at
        `
		_, err = tx.Exec(ctx, upsertQuery, id, newDelta, time.Now())
		if err != nil {
			return fmt.Errorf("failed to upsert counter %s: %w", id, err)
		}
	}

	// Обрабатываем gauges (просто перезаписываем)
	if len(gauges) > 0 {
		batch := &pgx.Batch{}
		query := `
            INSERT INTO metrics (id, type, delta, value, updated_at)
            VALUES ($1, 'gauge', NULL, $2, $3)
            ON CONFLICT (id) 
            DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
        `

		for id, metric := range gauges {
			batch.Queue(query, id, metric.Value, time.Now())
		}

		br := tx.SendBatch(ctx, batch)
		defer br.Close()

		// Проверяем результаты
		for range gauges {
			_, err := br.Exec()
			if err != nil {
				return fmt.Errorf("failed to execute batch: %w", err)
			}
		}
	}

	// Коммитим транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadData не используется для PostgreSQL (данные уже в БД)
func (r *PostgresRepository) LoadData(data map[string]*model.Metrics) {
	// Для PostgreSQL эта операция не нужна, данные уже персистентны
	// Можно реализовать bulk insert при необходимости
}

// Ping проверяет соединение с БД
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
