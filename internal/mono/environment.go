package mono

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Environment struct {
	ID            int64
	Path          string
	DockerProject sql.NullString
	CreatedAt     time.Time
}

func (db *DB) InsertEnvironment(path, dockerProject string) (int64, error) {
	var dp sql.NullString
	if dockerProject != "" {
		dp = sql.NullString{String: dockerProject, Valid: true}
	}

	result, err := db.conn.Exec(
		`INSERT INTO environments (path, docker_project) VALUES (?, ?)`,
		path, dp,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert environment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

func (db *DB) GetEnvironmentByPath(path string) (*Environment, error) {
	row := db.conn.QueryRow(
		`SELECT id, path, docker_project, created_at FROM environments WHERE path = ?`,
		path,
	)

	var e Environment
	err := row.Scan(&e.ID, &e.Path, &e.DockerProject, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("environment not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	return &e, nil
}

func (db *DB) ListEnvironments() ([]*Environment, error) {
	rows, err := db.conn.Query(
		`SELECT id, path, docker_project, created_at FROM environments ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	var environments []*Environment
	for rows.Next() {
		var e Environment
		err := rows.Scan(&e.ID, &e.Path, &e.DockerProject, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}
		environments = append(environments, &e)
	}

	return environments, rows.Err()
}

func (db *DB) EnvironmentExists(path string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM environments WHERE path = ?`,
		path,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check environment existence: %w", err)
	}
	return count > 0, nil
}

func (db *DB) DeleteEnvironment(path string) error {
	result, err := db.conn.Exec(
		`DELETE FROM environments WHERE path = ?`,
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return errors.New("environment not found")
	}

	return nil
}
