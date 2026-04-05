package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type CheckResult struct {
	ID         int64
	Endpoint   string
	URL        string
	StatusCode int
	Duration   int64 // milliseconds
	Success    bool
	Error      string
	CheckedAt  time.Time
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint TEXT NOT NULL,
			url TEXT NOT NULL,
			status_code INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			success BOOLEAN DEFAULT 0,
			error TEXT DEFAULT '',
			checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_checks_endpoint ON checks(endpoint);
		CREATE INDEX IF NOT EXISTS idx_checks_time ON checks(checked_at);
	`)
	return err
}

func (s *Store) SaveCheck(r CheckResult) error {
	_, err := s.db.Exec(
		"INSERT INTO checks (endpoint, url, status_code, duration_ms, success, error, checked_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		r.Endpoint, r.URL, r.StatusCode, r.Duration, r.Success, r.Error, r.CheckedAt,
	)
	return err
}

func (s *Store) GetRecentChecks(endpoint string, hours int) ([]CheckResult, error) {
	rows, err := s.db.Query(
		"SELECT id, endpoint, url, status_code, duration_ms, success, error, checked_at FROM checks WHERE endpoint = ? AND checked_at > datetime('now', ?) ORDER BY checked_at DESC",
		endpoint, fmt.Sprintf("-%d hours", hours),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CheckResult
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.ID, &r.Endpoint, &r.URL, &r.StatusCode, &r.Duration, &r.Success, &r.Error, &r.CheckedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) GetUptime(endpoint string, hours int) (float64, error) {
	var total, success int
	err := s.db.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) FROM checks WHERE endpoint = ? AND checked_at > datetime('now', ?)",
		endpoint, fmt.Sprintf("-%d hours", hours),
	).Scan(&total, &success)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	return float64(success) / float64(total) * 100, nil
}

func (s *Store) GetAllStatus() (map[string]CheckResult, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.endpoint, c.url, c.status_code, c.duration_ms, c.success, c.error, c.checked_at
		FROM checks c
		INNER JOIN (
			SELECT endpoint, MAX(checked_at) as max_time
			FROM checks
			GROUP BY endpoint
		) latest ON c.endpoint = latest.endpoint AND c.checked_at = latest.max_time
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]CheckResult)
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.ID, &r.Endpoint, &r.URL, &r.StatusCode, &r.Duration, &r.Success, &r.Error, &r.CheckedAt); err != nil {
			return nil, err
		}
		result[r.Endpoint] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// PurgeOldChecks deletes check records older than the given number of days.
// Returns the number of rows deleted.
func (s *Store) PurgeOldChecks(retentionDays int) (int64, error) {
	result, err := s.db.Exec(
		"DELETE FROM checks WHERE checked_at < datetime('now', ?)",
		fmt.Sprintf("-%d days", retentionDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) GetAllEndpoints() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT endpoint FROM checks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, name)
	}
	return endpoints, rows.Err()
}

func (s *Store) GetAverageResponseTime(endpoint string, hours int) (float64, error) {
	var avg float64
	err := s.db.QueryRow(
		"SELECT COALESCE(AVG(duration_ms), 0) FROM checks WHERE endpoint = ? AND checked_at > datetime('now', ?)",
		endpoint, fmt.Sprintf("-%d hours", hours),
	).Scan(&avg)
	return avg, err
}

func (s *Store) GetRecentCheckCount(endpoint string, hours int) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM checks WHERE endpoint = ? AND checked_at > datetime('now', ?)",
		endpoint, fmt.Sprintf("-%d hours", hours),
	).Scan(&count)
	return count, err
}
