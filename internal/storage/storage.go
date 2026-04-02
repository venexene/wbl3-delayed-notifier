package storage

import (
    "context"
	"time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
    Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Postgres, error) {
    pool, err := pgxpool.New(ctx, dsn)
    if err != nil {
        return nil, err
    }

    return &Postgres{Pool: pool}, nil
}


type Notification struct {
    ID        string
    Target    string
    Message   string
    SendAt    time.Time
    Status    string
    Retry     int
    CreatedAt time.Time
}


func (p *Postgres) Create(ctx context.Context, n Notification) error {
    query := `
        INSERT INTO notifications (id, target, message, send_at, status)
        VALUES ($1, $2, $3, $4, $5)
    `
    _, err := p.Pool.Exec(ctx, query,
        n.ID, n.Target, n.Message, n.SendAt, n.Status,
    )
    return err
}


func (p *Postgres) GetByID(ctx context.Context, id string) (*Notification, error) {
    query := `
        SELECT id, target, message, send_at, status, retry, created_at
        FROM notifications
        WHERE id = $1
    `

    var n Notification

    err := p.Pool.QueryRow(ctx, query, id).Scan(
        &n.ID,
        &n.Target,
        &n.Message,
        &n.SendAt,
        &n.Status,
        &n.Retry,
        &n.CreatedAt,
    )

    if err != nil {
        return nil, err
    }

    return &n, nil
}


func (p *Postgres) Cancel(ctx context.Context, id string) error {
    query := `
        UPDATE notifications
        SET status = 'canceled'
        WHERE id = $1
    `
    _, err := p.Pool.Exec(ctx, query, id)
    return err
}


func (p *Postgres) GetPending(ctx context.Context) ([]Notification, error) {
	rows, err := p.Pool.Query(ctx, `
        SELECT id, target, message, send_at, status, retry
        FROM notifications
        WHERE status = 'pending' AND send_at <= NOW()
        ORDER BY send_at ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Target, &n.Message, &n.SendAt, &n.Status, &n.Retry); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
}


func (p *Postgres) MarkSent(ctx context.Context, id string) error {
	_, err := p.Pool.Exec(ctx, `UPDATE notifications SET status='sent' WHERE id=$1`, id)
	return err
}


func (p *Postgres) IncrementRetry(ctx context.Context, id string) error {
	_, err := p.Pool.Exec(ctx, `
        UPDATE notifications 
        SET retry = retry + 1 
        WHERE id = $1
    `, id)
	return err
}