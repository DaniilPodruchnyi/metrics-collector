package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DaniilPodruchnyi/metrics-collector/internal/model"
	"github.com/DaniilPodruchnyi/metrics-collector/internal/retry"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// isRetryableDBError проверяет, является ли ошибка БД retriable
func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Class 08 — Connection Exception
		switch pgErr.Code {
		case pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
			pgerrcode.TransactionResolutionUnknown,
			pgerrcode.ProtocolViolation:
			return true
		}
	}

	// Проверяем другие retriable ошибки
	return retry.IsRetryable(err)
}

// Store сохраняет метрику в БД с retry
func (r *PostgresRepository) Store(metric *model.Metrics) error {
	retryCfg := retry.DefaultConfig()

	return retry.DoWithContext(context.Background(), func(ctx context.Context) error {
		err := r.storeWithContext(ctx, metric)
		// Проверяем, нужно ли делать retry
		if err != nil && !isRetryableDBError(err) {
			return nil // Не делаем retry для non-retriable ошибок
		}
		return err
	}, retryCfg)
}

// storeWithContext выполняет сохранение с контекстом
func (r *PostgresRepository) storeWithContext(ctx context.Context, metric *model.Metrics) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	return err
}

// Get возвращает метрику по имени с retry
func (r *PostgresRepository) Get(name string) (*model.Metrics, bool, error) {
	var metric *model.Metrics
	var found bool
	var finalErr error

	retryCfg := retry.DefaultConfig()

	err := retry.DoWithContext(context.Background(), func(ctx context.Context) error {
		var err error
		metric, found, err = r.getWithContext(ctx, name)

		// Проверяем, нужно ли делать retry
		if err != nil && !isRetryableDBError(err) {
			finalErr = err
			return nil // Не делаем retry для non-retriable ошибок
		}

		return err
	}, retryCfg)

	if err != nil {
		return nil, false, err
	}

	if finalErr != nil {
		return nil, false, finalErr
	}

	return metric, found, nil
}

// getWithContext выполняет получение с контекстом
func (r *PostgresRepository) getWithContext(ctx context.Context, name string) (*model.Metrics, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
			return nil, false, nil
		}
		return nil, false, err
	}

	return &metric, true, nil
}

// GetAll возвращает все метрики с retry
func (r *PostgresRepository) GetAll() (map[string]*model.Metrics, error) {
	var result map[string]*model.Metrics
	var finalErr error

	retryCfg := retry.DefaultConfig()

	err := retry.DoWithContext(context.Background(), func(ctx context.Context) error {
		var err error
		result, err = r.getAllWithContext(ctx)

		// Проверяем, нужно ли делать retry
		if err != nil && !isRetryableDBError(err) {
			finalErr = err
			return nil // Не делаем retry для non-retriable ошибок
		}

		return err
	}, retryCfg)

	if err != nil {
		return make(map[string]*model.Metrics), err
	}

	if finalErr != nil {
		return make(map[string]*model.Metrics), finalErr
	}

	return result, nil
}

// getAllWithContext выполняет получение всех метрик с контекстом
func (r *PostgresRepository) getAllWithContext(ctx context.Context) (map[string]*model.Metrics, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
        SELECT id, type, delta, value
        FROM metrics
        ORDER BY id
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		result[metric.ID] = &metric
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// StoreBatch сохраняет множество метрик в одной транзакции с retry
func (r *PostgresRepository) StoreBatch(metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	retryCfg := retry.DefaultConfig()

	return retry.DoWithContext(context.Background(), func(ctx context.Context) error {
		err := r.storeBatchWithContext(ctx, metrics)

		// Проверяем, нужно ли делать retry
		if err != nil && !isRetryableDBError(err) {
			return nil // Не делаем retry для non-retriable ошибок
		}

		return err
	}, retryCfg)
}

// storeBatchWithContext выполняет batch сохранение с контекстом
func (r *PostgresRepository) storeBatchWithContext(ctx context.Context, metrics []model.Metrics) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, metric := range metrics {
		if metric.MType == model.Counter {
			var existingDelta int64
			query := `SELECT COALESCE(delta, 0) FROM metrics WHERE id = $1 AND type = 'counter'`
			err := tx.QueryRow(ctx, query, metric.ID).Scan(&existingDelta)

			var newDelta int64
			if err == nil {
				newDelta = existingDelta + *metric.Delta
			} else if errors.Is(err, pgx.ErrNoRows) {
				newDelta = *metric.Delta
			} else {
				return fmt.Errorf("failed to query counter %s: %w", metric.ID, err)
			}

			upsertQuery := `
                INSERT INTO metrics (id, type, delta, value, updated_at)
                VALUES ($1, 'counter', $2, NULL, $3)
                ON CONFLICT (id) 
                DO UPDATE SET 
                    delta = EXCLUDED.delta, 
                    type = EXCLUDED.type,
                    updated_at = EXCLUDED.updated_at
            `
			_, err = tx.Exec(ctx, upsertQuery, metric.ID, newDelta, time.Now())
			if err != nil {
				return fmt.Errorf("failed to upsert counter %s: %w", metric.ID, err)
			}
		} else {
			upsertQuery := `
                INSERT INTO metrics (id, type, delta, value, updated_at)
                VALUES ($1, 'gauge', NULL, $2, $3)
                ON CONFLICT (id) 
                DO UPDATE SET 
                    value = EXCLUDED.value,
                    type = EXCLUDED.type,
                    updated_at = EXCLUDED.updated_at
            `
			_, err := tx.Exec(ctx, upsertQuery, metric.ID, metric.Value, time.Now())
			if err != nil {
				return fmt.Errorf("failed to upsert gauge %s: %w", metric.ID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadData загружает данные из map в PostgreSQL
func (r *PostgresRepository) LoadData(data map[string]*model.Metrics) error {
	if len(data) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for LoadData: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, metric := range data {
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
		_, err := tx.Exec(ctx, query,
			metric.ID,
			metric.MType,
			metric.Delta,
			metric.Value,
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("failed to load metric %s: %w", metric.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit LoadData transaction: %w", err)
	}

	return nil
}

// Ping проверяет соединение с БД с retry
func (r *PostgresRepository) Ping(ctx context.Context) error {
	retryCfg := retry.DefaultConfig()

	return retry.DoWithContext(ctx, func(ctx context.Context) error {
		err := r.pool.Ping(ctx)

		// Проверяем, нужно ли делать retry
		if err != nil && !isRetryableDBError(err) {
			return nil // Не делаем retry для non-retriable ошибок
		}

		return err
	}, retryCfg)
}
