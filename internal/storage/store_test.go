package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GolovachevS/pr-reviewer-service/internal/domain"
	"github.com/GolovachevS/pr-reviewer-service/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestStoreCreateTeamSuccess(t *testing.T) {
	ctx := context.Background()
	team := domain.Team{
		TeamName: "core",
		Members: []domain.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: false},
		},
	}

	tx := &fakeTx{}
	beginCalled := false
	pool := &fakePool{
		beginTxFunc: func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
			beginCalled = true
			return tx, nil
		},
	}

	teamExistsChecked := false
	tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "FROM teams") && !teamExistsChecked {
			teamExistsChecked = true
			return fakeRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
		}
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected query row: %s", sql) }}
	}

	insertCount := 0
	tx.execFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		insertCount++
		return pgconn.CommandTag{}, nil
	}

	tx.commitFunc = func(context.Context) error { return nil }
	tx.rollbackFunc = func(context.Context) error { return pgx.ErrTxClosed }

	pool.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		return fakeRow{scan: func(dest ...any) error {
			*(dest[0].(*string)) = team.TeamName
			return nil
		}}
	}

	pool.queryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		rows := [][]any{
			{"u1", "Alice", true},
			{"u2", "Bob", false},
		}
		return &fakeRows{data: rows}, nil
	}

	store := New(pool)
	got, err := store.CreateTeam(ctx, team)
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	if !beginCalled || insertCount != len(team.Members)+1 {
		t.Fatalf("expected inserts for team and members, got %d", insertCount)
	}
	if len(got.Members) != len(team.Members) {
		t.Fatalf("expected team members returned")
	}
}

func TestStoreCreateTeamExists(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{}
	tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		return fakeRow{scan: func(dest ...any) error {
			*(dest[0].(*string)) = args[0].(string)
			return nil
		}}
	}
	tx.rollbackFunc = func(context.Context) error { return nil }

	pool := &fakePool{
		beginTxFunc: func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	store := New(pool)

	_, err := store.CreateTeam(ctx, domain.Team{TeamName: "core"})
	if err == nil {
		t.Fatalf("expected error when team exists")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeTeamExists {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreCreatePullRequestAssignsReviewers(t *testing.T) {
	ctx := context.Background()
	input := service.CreatePullRequestInput{
		PullRequestID:   "pr-1",
		PullRequestName: "Add search",
		AuthorID:        "author",
	}

	candidateRows := &fakeRows{data: [][]any{{"author"}, {"u2"}, {"u3"}}}
	tx := &fakeTx{}
	tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "FROM users") {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*string)) = "payments"
				return nil
			}}
		}
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected query row: %s", sql) }}
	}
	tx.execFunc = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, nil
	}
	tx.queryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		return candidateRows, nil
	}
	tx.commitFunc = func(context.Context) error { return nil }
	tx.rollbackFunc = func(context.Context) error { return nil }

	pool := &fakePool{
		beginTxFunc: func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}

	pool.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "FROM pull_requests") {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*string)) = input.PullRequestID
				*(dest[1].(*string)) = input.PullRequestName
				*(dest[2].(*string)) = input.AuthorID
				*(dest[3].(*string)) = "OPEN"
				t := time.Now()
				*(dest[4].(*time.Time)) = t
				return nil
			}}
		}
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected query row: %s", sql) }}
	}
	pool.queryFunc = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		return &fakeRows{data: [][]any{{"u2"}, {"u3"}}}, nil
	}

	store := New(pool)
	var captured []string
	pr, err := store.CreatePullRequest(ctx, input, func(ids []string, limit int) []string {
		captured = append([]string(nil), ids...)
		return []string{"u2", "u3"}
	})
	if err != nil {
		t.Fatalf("CreatePullRequest error: %v", err)
	}
	if contains(captured, "author") {
		t.Fatalf("author should be excluded from candidates: %v", captured)
	}
	if len(pr.Assigned) != 2 {
		t.Fatalf("expected two reviewers, got %v", pr.Assigned)
	}
}

func TestStoreReassignReviewerMerged(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{}
	tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if strings.Contains(sql, "status FROM pull_requests") {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*string)) = "MERGED"
				return nil
			}}
		}
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected query row: %s", sql) }}
	}
	tx.rollbackFunc = func(context.Context) error { return nil }

	pool := &fakePool{
		beginTxFunc: func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	store := New(pool)

	_, _, err := store.ReassignReviewer(ctx, "pr-1", "old", func([]string) (string, bool) {
		return "", false
	})
	if err == nil {
		t.Fatalf("expected error for merged PR")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodePRMerged {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- test fakes ---

type fakePool struct {
	beginTxFunc  func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	execFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (f *fakePool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return f.beginTxFunc(ctx, txOptions)
}

func (f *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFunc == nil {
		return nil, fmt.Errorf("unexpected Query: %s", sql)
	}
	return f.queryFunc(ctx, sql, args...)
}

func (f *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFunc == nil {
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected QueryRow: %s", sql) }}
	}
	return f.queryRowFunc(ctx, sql, args...)
}

func (f *fakePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFunc == nil {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", sql)
	}
	return f.execFunc(ctx, sql, args...)
}

type fakeTx struct {
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	commitFunc   func(ctx context.Context) error
	rollbackFunc func(ctx context.Context) error
}

func (f *fakeTx) Begin(context.Context) (pgx.Tx, error) { panic("not implemented") }

func (f *fakeTx) Conn() *pgx.Conn { return nil }

func (f *fakeTx) Commit(ctx context.Context) error {
	if f.commitFunc != nil {
		return f.commitFunc(ctx)
	}
	return nil
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	if f.rollbackFunc != nil {
		return f.rollbackFunc(ctx)
	}
	return nil
}

func (f *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("not implemented")
}

func (f *fakeTx) LargeObjects() pgx.LargeObjects { panic("not implemented") }

func (f *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("not implemented") }

func (f *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("not implemented")
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFunc == nil {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", sql)
	}
	return f.execFunc(ctx, sql, args...)
}

func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFunc == nil {
		return nil, fmt.Errorf("unexpected Query: %s", sql)
	}
	return f.queryFunc(ctx, sql, args...)
}

func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFunc == nil {
		return fakeRow{scan: func(dest ...any) error { return fmt.Errorf("unexpected QueryRow: %s", sql) }}
	}
	return f.queryRowFunc(ctx, sql, args...)
}

type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scan(dest...)
}

type fakeRows struct {
	data [][]any
	idx  int
	err  error
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error { return r.err }

func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.data[r.idx-1]
	for i := range dest {
		switch v := dest[i].(type) {
		case *string:
			*v = row[i].(string)
		case *bool:
			*v = row[i].(bool)
		case *time.Time:
			*v = row[i].(time.Time)
		default:
			return fmt.Errorf("unsupported scan dest")
		}
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }

func (r *fakeRows) RawValues() [][]byte { return nil }

func (r *fakeRows) Conn() *pgx.Conn { return nil }

func contains(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}
