package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// OperationalSnapshot keeps every existing top-level search field. New sections
// expose only resource counters and a fixed HTTP vocabulary.
type OperationalSnapshot struct {
	SearchSnapshot
	HTTP         HTTPSnapshot `json:"http"`
	DatabasePool PoolSnapshot `json:"database_pool"`
}
type PoolSnapshot struct {
	MaxConnections          int32 `json:"max_connections"`
	TotalConnections        int32 `json:"total_connections"`
	AcquiredConnections     int32 `json:"acquired_connections"`
	IdleConnections         int32 `json:"idle_connections"`
	ConstructingConnections int32 `json:"constructing_connections"`
	Acquires                int64 `json:"acquires"`
	EmptyAcquires           int64 `json:"empty_acquires"`
	CanceledAcquires        int64 `json:"canceled_acquires"`
	AcquireDurationMillis   int64 `json:"acquire_duration_millis"`
}

func SnapshotPool(s *pgxpool.Stat) PoolSnapshot {
	if s == nil {
		return PoolSnapshot{}
	}
	return PoolSnapshot{MaxConnections: s.MaxConns(), TotalConnections: s.TotalConns(), AcquiredConnections: s.AcquiredConns(), IdleConnections: s.IdleConns(), ConstructingConnections: s.ConstructingConns(), Acquires: s.AcquireCount(), EmptyAcquires: s.EmptyAcquireCount(), CanceledAcquires: s.CanceledAcquireCount(), AcquireDurationMillis: int64(s.AcquireDuration() / time.Millisecond)}
}
