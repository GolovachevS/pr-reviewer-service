package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/GolovachevS/pr-reviewer-service/internal/domain"
	"github.com/GolovachevS/pr-reviewer-service/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Store implements the service.Repository interface using PostgreSQL.
type pgxPool interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Store struct {
	pool pgxPool
}

func New(pool pgxPool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Team{}, err
	}
	defer rollbackTx(ctx, tx)

	var existing string
	if scanErr := tx.QueryRow(ctx, "SELECT team_name FROM teams WHERE team_name=$1", team.TeamName).Scan(&existing); scanErr == nil {
		return domain.Team{}, domain.NewTeamExistsError(nil)
	} else if !errors.Is(scanErr, pgx.ErrNoRows) {
		return domain.Team{}, scanErr
	}

	if _, execErr := tx.Exec(ctx, "INSERT INTO teams(team_name) VALUES($1)", team.TeamName); execErr != nil {
		return domain.Team{}, execErr
	}

	for _, member := range team.Members {
		_, err = tx.Exec(
			ctx,
			`INSERT INTO users(user_id, username, team_name, is_active)
			 VALUES($1, $2, $3, $4)
			 ON CONFLICT (user_id)
			 DO UPDATE SET username = EXCLUDED.username,
			               team_name = EXCLUDED.team_name,
			               is_active = EXCLUDED.is_active,
			               updated_at = NOW()`,
			member.UserID,
			member.Username,
			team.TeamName,
			member.IsActive,
		)
		if err != nil {
			return domain.Team{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Team{}, err
	}

	return s.GetTeam(ctx, team.TeamName)
}

func (s *Store) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	if err := s.ensureTeamExists(ctx, teamName); err != nil {
		return domain.Team{}, err
	}

	members, err := s.listTeamMembers(ctx, teamName)
	if err != nil {
		return domain.Team{}, err
	}

	return domain.Team{TeamName: teamName, Members: members}, nil
}

func (s *Store) SetUserActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	var user domain.User
	row := s.pool.QueryRow(ctx, `UPDATE users SET is_active=$2, updated_at=NOW() WHERE user_id=$1 RETURNING user_id, username, team_name, is_active`, userID, isActive)
	if err := row.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.NewNotFoundError("user not found", err)
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) CreatePullRequest(ctx context.Context, input service.CreatePullRequestInput, pick func([]string, int) []string) (domain.PullRequest, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.PullRequest{}, err
	}
	defer rollbackTx(ctx, tx)

	var teamName string
	row := tx.QueryRow(ctx, `SELECT team_name FROM users WHERE user_id=$1`, input.AuthorID)
	if scanErr := row.Scan(&teamName); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return domain.PullRequest{}, domain.NewNotFoundError("author not found", scanErr)
		}
		return domain.PullRequest{}, scanErr
	}

	insertPR := `INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id)
		VALUES($1, $2, $3)`
	if _, execErr := tx.Exec(ctx, insertPR, input.PullRequestID, input.PullRequestName, input.AuthorID); execErr != nil {
		if isUniqueViolation(execErr) {
			return domain.PullRequest{}, domain.NewPRExistsError(execErr)
		}
		return domain.PullRequest{}, execErr
	}

	excluded := []string{input.AuthorID}
	candidates, err := s.listActiveTeamMembersTx(ctx, tx, teamName, excluded)
	if err != nil {
		return domain.PullRequest{}, err
	}

	reviewers := pick(candidates, 2)
	for _, reviewerID := range reviewers {
		if _, execErr := tx.Exec(ctx, `INSERT INTO pull_request_reviewers(pull_request_id, reviewer_id) VALUES($1, $2)`, input.PullRequestID, reviewerID); execErr != nil {
			return domain.PullRequest{}, execErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PullRequest{}, err
	}

	return s.GetPullRequest(ctx, input.PullRequestID)
}

func (s *Store) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	row := s.pool.QueryRow(ctx, `UPDATE pull_requests
		SET status='MERGED',
		    merged_at = COALESCE(merged_at, NOW())
		WHERE pull_request_id=$1
		RETURNING pull_request_id, pull_request_name, author_id, status, created_at, merged_at`, prID)

	pr, err := scanPullRequestRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PullRequest{}, domain.NewNotFoundError("pull request not found", err)
		}
		return domain.PullRequest{}, err
	}

	assigned, err := s.listAssignedReviewers(ctx, pr.PullRequestID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	pr.Assigned = assigned

	return pr, nil
}

func (s *Store) ReassignReviewer(ctx context.Context, prID, oldUserID string, pick func([]string) (string, bool)) (domain.PullRequest, string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.PullRequest{}, "", err
	}
	defer rollbackTx(ctx, tx)

	var status string
	row := tx.QueryRow(ctx, `SELECT status FROM pull_requests WHERE pull_request_id=$1 FOR UPDATE`, prID)
	if scanErr := row.Scan(&status); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return domain.PullRequest{}, "", domain.NewNotFoundError("pull request not found", scanErr)
		}
		return domain.PullRequest{}, "", scanErr
	}

	if status == "MERGED" {
		return domain.PullRequest{}, "", domain.NewPRMergedError()
	}

	var reviewerTeam string
	row = tx.QueryRow(ctx, `SELECT team_name FROM users WHERE user_id=$1`, oldUserID)
	if scanErr := row.Scan(&reviewerTeam); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return domain.PullRequest{}, "", domain.NewNotFoundError("reviewer not found", scanErr)
		}
		return domain.PullRequest{}, "", scanErr
	}

	var exists int
	if scanErr := tx.QueryRow(ctx, `SELECT 1 FROM pull_request_reviewers WHERE pull_request_id=$1 AND reviewer_id=$2`, prID, oldUserID).Scan(&exists); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return domain.PullRequest{}, "", domain.NewNotAssignedError()
		}
		return domain.PullRequest{}, "", scanErr
	}

	assigned, err := s.listAssignedReviewersTx(ctx, tx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	exclude := append([]string{oldUserID}, assigned...)
	candidates, err := s.listActiveTeamMembersTx(ctx, tx, reviewerTeam, exclude)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	chosen, ok := pick(candidates)
	if !ok {
		return domain.PullRequest{}, "", domain.NewNoCandidateError()
	}

	if _, execErr := tx.Exec(ctx, `DELETE FROM pull_request_reviewers WHERE pull_request_id=$1 AND reviewer_id=$2`, prID, oldUserID); execErr != nil {
		return domain.PullRequest{}, "", execErr
	}

	if _, execErr := tx.Exec(ctx, `INSERT INTO pull_request_reviewers(pull_request_id, reviewer_id) VALUES($1, $2)`, prID, chosen); execErr != nil {
		return domain.PullRequest{}, "", execErr
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return domain.PullRequest{}, "", commitErr
	}

	pr, err := s.GetPullRequest(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	return pr, chosen, nil
}

func (s *Store) GetUserReviews(ctx context.Context, userID string) (domain.UserReviews, error) {
	rows, err := s.pool.Query(ctx, `SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pull_request_reviewers r
		JOIN pull_requests pr ON pr.pull_request_id = r.pull_request_id
		WHERE r.reviewer_id=$1
		ORDER BY pr.created_at DESC`, userID)
	if err != nil {
		return domain.UserReviews{}, err
	}
	defer rows.Close()

	var prs []domain.PullRequestShort
	for rows.Next() {
		var item domain.PullRequestShort
		if err := rows.Scan(&item.PullRequestID, &item.PullRequestName, &item.AuthorID, &item.Status); err != nil {
			return domain.UserReviews{}, err
		}
		prs = append(prs, item)
	}

	if err := rows.Err(); err != nil {
		return domain.UserReviews{}, err
	}

	return domain.UserReviews{UserID: userID, PullRequests: prs}, nil
}

// Helper functions

func (s *Store) ensureTeamExists(ctx context.Context, teamName string) error {
	row := s.pool.QueryRow(ctx, "SELECT team_name FROM teams WHERE team_name=$1", teamName)
	var name string
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NewNotFoundError("team not found", err)
		}
		return err
	}
	return nil
}

func (s *Store) listTeamMembers(ctx context.Context, teamName string) ([]domain.TeamMember, error) {
	rows, err := s.pool.Query(ctx, `SELECT user_id, username, is_active FROM users WHERE team_name=$1 ORDER BY username`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.TeamMember
	for rows.Next() {
		var member domain.TeamMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.IsActive); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}

func (s *Store) listActiveTeamMembersTx(ctx context.Context, tx pgx.Tx, teamName string, excludes []string) ([]string, error) {
	exclusion := make(map[string]struct{}, len(excludes))
	for _, id := range excludes {
		if id != "" {
			exclusion[id] = struct{}{}
		}
	}

	rows, err := tx.Query(ctx, `SELECT user_id FROM users WHERE team_name=$1 AND is_active=true`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if _, skip := exclusion[id]; skip {
			continue
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

func (s *Store) listAssignedReviewers(ctx context.Context, prID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT reviewer_id FROM pull_request_reviewers WHERE pull_request_id=$1 ORDER BY reviewer_id`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviewers []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, id)
	}
	return reviewers, rows.Err()
}

func (s *Store) listAssignedReviewersTx(ctx context.Context, tx pgx.Tx, prID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT reviewer_id FROM pull_request_reviewers WHERE pull_request_id=$1`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviewers []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		reviewers = append(reviewers, id)
	}
	return reviewers, rows.Err()
}

func (s *Store) GetPullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	row := s.pool.QueryRow(ctx, `SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests WHERE pull_request_id=$1`, prID)
	pr, err := scanPullRequestRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PullRequest{}, domain.NewNotFoundError("pull request not found", err)
		}
		return domain.PullRequest{}, err
	}

	reviewers, err := s.listAssignedReviewers(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	pr.Assigned = reviewers
	return pr, nil
}

func scanPullRequestRow(row pgx.Row) (domain.PullRequest, error) {
	var pr domain.PullRequest
	var mergedAt sql.NullTime
	if err := row.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &mergedAt); err != nil {
		return domain.PullRequest{}, err
	}
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}
	return pr, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func rollbackTx(ctx context.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		_ = err // rollback is best-effort; nothing else to do here
	}
}
