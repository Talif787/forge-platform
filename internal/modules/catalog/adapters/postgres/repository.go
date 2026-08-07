package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/modules/catalog/domain"
	"github.com/forge-platform/forge/internal/platform/apperr"
	"github.com/forge-platform/forge/internal/platform/id"
	platformpg "github.com/forge-platform/forge/internal/platform/postgres"
)

type Repository struct{ db platformpg.DBTX }

const serviceColumns = `id::text, tenant_id::text, name, description, tier, lifecycle, repository_url, owning_team, on_call_ref, version, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, s *domain.Service) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO catalog_services
			(id, tenant_id, name, description, tier, lifecycle, repository_url, owning_team, on_call_ref, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		s.ID().String(), s.TenantID().String(), s.Name().String(), s.Description(),
		s.Tier().Int(), string(s.Lifecycle()), s.Repository(),
		s.Ownership().OwningTeam, s.Ownership().OnCallRef,
		s.Version(), s.CreatedAt(), s.UpdatedAt(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrNameConflict
		}
		return fmt.Errorf("insert service: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO catalog_ownership_history (id, service_id, owning_team, on_call_ref, valid_from)
		VALUES ($1,$2,$3,$4,$5)`,
		id.New().String(), s.ID().String(), s.Ownership().OwningTeam, s.Ownership().OnCallRef, s.CreatedAt())
	if err != nil {
		return fmt.Errorf("insert ownership history: %w", err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, s *domain.Service, expectedVersion int64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE catalog_services
		   SET description=$1, tier=$2, lifecycle=$3, repository_url=$4,
		       owning_team=$5, on_call_ref=$6, version=$7, updated_at=$8
		 WHERE id=$9::uuid AND version=$10`,
		s.Description(), s.Tier().Int(), string(s.Lifecycle()), s.Repository(),
		s.Ownership().OwningTeam, s.Ownership().OnCallRef, s.Version(), s.UpdatedAt(),
		s.ID().String(), expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.FindByID(ctx, s.ID()); errors.Is(err, domain.ErrServiceNotFound) {
			return domain.ErrServiceNotFound
		}
		return domain.ErrVersionConflict
	}
	if err := r.recordOwnership(ctx, s); err != nil {
		return err
	}
	return nil
}

func (r *Repository) recordOwnership(ctx context.Context, s *domain.Service) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE catalog_ownership_history
		   SET valid_to = $2
		 WHERE service_id = $1::uuid AND valid_to IS NULL
		   AND (owning_team, on_call_ref) IS DISTINCT FROM ($3, $4)`,
		s.ID().String(), s.UpdatedAt(), s.Ownership().OwningTeam, s.Ownership().OnCallRef); err != nil {
		return fmt.Errorf("close ownership interval: %w", err)
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO catalog_ownership_history (id, service_id, owning_team, on_call_ref, valid_from)
		SELECT $1, $2, $3, $4, $5
		 WHERE NOT EXISTS (
		   SELECT 1 FROM catalog_ownership_history
		    WHERE service_id = $2::uuid AND valid_to IS NULL
		      AND owning_team = $3 AND on_call_ref = $4)`,
		id.New().String(), s.ID().String(), s.Ownership().OwningTeam, s.Ownership().OnCallRef, s.UpdatedAt()); err != nil {
		return fmt.Errorf("open ownership interval: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, sid domain.ServiceID) (*domain.Service, error) {
	row := r.db.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE id=$1::uuid`, sid.String())
	return scanService(row)
}

func (r *Repository) FindByTenantAndName(ctx context.Context, tenant domain.TenantID, name domain.ServiceName) (*domain.Service, error) {
	row := r.db.QueryRow(ctx, `SELECT `+serviceColumns+` FROM catalog_services WHERE tenant_id=$1::uuid AND name=$2`, tenant.String(), name.String())
	return scanService(row)
}

func (r *Repository) List(ctx context.Context, f app.ListFilter) ([]*domain.Service, string, error) {
	qb := newQueryBuilder(`SELECT ` + serviceColumns + ` FROM catalog_services`)
	if f.TenantID != nil {
		qb.where("tenant_id = %s::uuid", f.TenantID.String())
	}
	if f.Lifecycle != nil {
		qb.where("lifecycle = %s", string(*f.Lifecycle))
	}
	if f.Tier != nil {
		qb.where("tier = %s", f.Tier.Int())
	}

	sortCol, valCast := "created_at", "timestamptz"
	if f.Sort == app.SortName {
		sortCol, valCast = "name", "text"
	}
	cmp, dir := ">", "ASC"
	if f.Order == app.OrderDesc {
		cmp, dir = "<", "DESC"
	}
	if f.Cursor != "" {
		cur, err := decodeCursor(f.Cursor, f.Sort, f.Order)
		if err != nil {
			return nil, "", err
		}
		qb.where(fmt.Sprintf("(%s, id) %s (%%s::%s, %%s::uuid)", sortCol, cmp, valCast), cur.Value, cur.ID)
	}
	qb.raw(fmt.Sprintf(" ORDER BY %s %s, id %s LIMIT %d", sortCol, dir, dir, f.Limit+1))

	rows, err := r.db.Query(ctx, qb.sql(), qb.args()...)
	if err != nil {
		return nil, "", fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []*domain.Service
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, "", err
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(services) > f.Limit {
		last := services[f.Limit-1]
		services = services[:f.Limit]
		next = encodeCursor(f, last)
	}
	return services, next, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanService(row scannable) (*domain.Service, error) {
	var (
		sid, tenantID, name, description, lifecycle, repo, team, onCall string
		tier                                                            int
		version                                                         int64
		createdAt, updatedAt                                            time.Time
	)
	if err := row.Scan(&sid, &tenantID, &name, &description, &tier, &lifecycle, &repo, &team, &onCall, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrServiceNotFound
		}
		return nil, fmt.Errorf("scan service: %w", err)
	}
	dsid, err := domain.ServiceIDFromString(sid)
	if err != nil {
		return nil, corrupt(err)
	}
	dtid, err := domain.TenantIDFromString(tenantID)
	if err != nil {
		return nil, corrupt(err)
	}
	dname, err := domain.NewServiceName(name)
	if err != nil {
		return nil, corrupt(err)
	}
	dtier, err := domain.NewTier(tier)
	if err != nil {
		return nil, corrupt(err)
	}
	dlifecycle, err := domain.ParseLifecycle(lifecycle)
	if err != nil {
		return nil, corrupt(err)
	}
	downership, err := domain.NewOwnership(team, onCall)
	if err != nil {
		return nil, corrupt(err)
	}
	return domain.Reconstitute(dsid, dtid, dname, description, dtier, dlifecycle, repo, downership, version, createdAt, updatedAt), nil
}

func corrupt(err error) error {
	return apperr.Internal("DATA_CORRUPTION", "stored service data is invalid").WithCause(err)
}

func encodeCursor(f app.ListFilter, s *domain.Service) string {
	value := s.CreatedAt().UTC().Format(time.RFC3339Nano)
	if f.Sort == app.SortName {
		value = s.Name().String()
	}
	c := cursor{Sort: string(f.Sort), Order: string(f.Order), Value: value, ID: s.ID().String()}
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

type cursor struct {
	Sort  string `json:"s"`
	Order string `json:"o"`
	Value string `json:"v"`
	ID    string `json:"id"`
}

func decodeCursor(encoded string, sort app.SortField, order app.SortOrder) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return cursor{}, apperr.Validation("INVALID_CURSOR", "cursor is malformed")
	}
	var c cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return cursor{}, apperr.Validation("INVALID_CURSOR", "cursor is malformed")
	}
	if c.Sort != string(sort) || c.Order != string(order) {
		return cursor{}, apperr.Validation("INVALID_CURSOR", "cursor does not match the requested sort or order")
	}
	return c, nil
}

// queryBuilder assembles a parameterized query, tracking placeholder indexes.
type queryBuilder struct {
	base    string
	clauses []string
	values  []any
}

func newQueryBuilder(base string) *queryBuilder { return &queryBuilder{base: base} }

func (q *queryBuilder) where(clauseFmt string, args ...any) {
	placeholders := make([]any, len(args))
	for i, a := range args {
		q.values = append(q.values, a)
		placeholders[i] = "$" + strconv.Itoa(len(q.values))
	}
	q.clauses = append(q.clauses, fmt.Sprintf(clauseFmt, placeholders...))
}

func (q *queryBuilder) raw(s string) { q.base += s }

func (q *queryBuilder) sql() string {
	sql := q.base
	if len(q.clauses) > 0 {
		sql = q.insertWhere(sql)
	}
	return sql
}

func (q *queryBuilder) insertWhere(sql string) string {
	where := " WHERE "
	for i, c := range q.clauses {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	// ORDER BY / LIMIT were appended via raw() after clauses; splice WHERE before them.
	idx := indexOfOrder(sql)
	if idx == -1 {
		return sql + where
	}
	return sql[:idx] + where + sql[idx:]
}

func indexOfOrder(sql string) int {
	for _, kw := range []string{" ORDER BY ", " LIMIT "} {
		if i := indexOf(sql, kw); i != -1 {
			return i
		}
	}
	return -1
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (q *queryBuilder) args() []any { return q.values }

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), used to translate a duplicate name into a domain
// conflict error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
