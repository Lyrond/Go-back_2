package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"spotlight.moonlight.net/internal/validator"
	"time"
)

type Game struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   uuid.UUID `json:"version"`
}

type GameModel struct {
	DB *pgxpool.Pool
}

func ValidateGame(v *validator.Validator, game *Game) {
	v.Check(game.Title != "", "title", "must be provided")
	v.Check(len(game.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(game.Year != 0, "year", "must be provided")
	v.Check(game.Year >= 1888, "year", "must be greater than 1888")
	v.Check(game.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(game.Genres != nil, "genres", "must be provided")
	v.Check(len(game.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(game.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(game.Genres), "genres", "must not contain duplicate values")
}

func (m GameModel) Insert(game *Game) error {
	query := `
INSERT INTO games (title, year, genres)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version`
	args := []interface{}{game.Title, game.Year, game.Genres}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return m.DB.QueryRow(ctx, query, args...).Scan(&game.ID, &game.CreatedAt, &game.Version)
}

func (m GameModel) Get(id int64) (*Game, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	query := `
SELECT id, created_at, title, year, genres, version
FROM games
WHERE id = $1`
	var game Game
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRow(ctx, query, id).Scan(
		&game.ID,
		&game.CreatedAt,
		&game.Title,
		&game.Year,
		&game.Genres,
		&game.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &game, nil
}

func (m GameModel) Update(game *Game) error {
	query := `
UPDATE games
SET title = $1, year = $2, genres = $4, version = uuid_generate_v4()
WHERE id = $5 AND version = $6
RETURNING version`
	args := []interface{}{
		game.Title,
		game.Year,
		game.Genres,
		game.ID,
		game.Version,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRow(ctx, query, args...).Scan(&game.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m GameModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}
	query := `
DELETE FROM games
WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (m GameModel) GetAll(title string, genres []string, filters Filters) ([]*Game, Metadata, error) {
	query := fmt.Sprintf(`
SELECT count(*) OVER(), id, created_at, title, year,  , genres, version
FROM games
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY %s %s, id ASC
LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []interface{}{title, genres, filters.limit(), filters.offset()}
	rows, err := m.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()
	totalRecords := 0
	games := []*Game{}
	for rows.Next() {
		var game Game
		err := rows.Scan(
			&totalRecords,
			&game.ID,
			&game.CreatedAt,
			&game.Title,
			&game.Year,
			&game.Genres,
			&game.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		games = append(games, &game)
	}
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	return games, metadata, nil
}
