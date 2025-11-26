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

// LoadData не используется для PostgreSQL (данные уже в БД)
func (r *PostgresRepository) LoadData(data map[string]*model.Metrics) {
	// Для PostgreSQL эта операция не нужна, данные уже персистентны
	// Можно реализовать bulk insert при необходимости
}

// Ping проверяет соединение с БД
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
