package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/forge-platform/forge/internal/modules/tenant/app"
	"github.com/forge-platform/forge/internal/modules/tenant/domain"
	"github.com/forge-platform/forge/internal/platform/apperr"
	platformpg "github.com/forge-platform/forge/internal/platform/postgres"
)

type Repository struct{ db platformpg.DBTX }

const tenantColumns = `id::text, name, slug, plan, status, max_services, version, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, t *domain.Tenant) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO tenants
			(id, name, slug, plan, status, max_services, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		t.ID().String(), t.Name().String(), t.Slug().String(), string(t.Plan()),
		string(t.Status()), t.Quota().MaxServices, t.Version(), t.CreatedAt(), t.UpdatedAt())
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrSlugConflict
		}
		return fmt.Errorf("insert tenant: %w", err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, t *domain.Tenant, expectedVersion int64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE tenants
		   SET name=$1, plan=$2, status=$3, max_services=$4, version=$5, updated_at=$6
		 WHERE id=$7::uuid AND version=$8`,
		t.Name().String(), string(t.Plan()), string(t.Status()), t.Quota().MaxServices,
		t.Version(), t.UpdatedAt(), t.ID().String(), expectedVersion)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.FindByID(ctx, t.ID()); errors.Is(err, domain.ErrTenantNotFound) {
			return domain.ErrTenantNotFound
		}
		return domain.ErrVersionConflict
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id domain.TenantID) (*domain.Tenant, error) {
	row := r.db.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE id=$1::uuid`, id.String())
	return scanTenant(row)
}

func (r *Repository) FindBySlug(ctx context.Context, slug domain.Slug) (*domain.Tenant, error) {
	row := r.db.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE slug=$1`, slug.String())
	return scanTenant(row)
}

func (r *Repository) List(ctx context.Context, f app.ListFilter) ([]*domain.Tenant, string, error) {
	conds := []string{}
	args := []any{}
	add := func(clause string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != nil {
		add("status = $%d", string(*f.Status))
	}
	if f.Plan != nil {
		add("plan = $%d", string(*f.Plan))
	}

	cmp, dir := ">", "ASC"
	if f.Order == app.OrderDesc {
		cmp, dir = "<", "DESC"
	}
	if f.Cursor != "" {
		cur, err := decodeCursor(f.Cursor, f.Order)
		if err != nil {
			return nil, "", err
		}
		args = append(args, cur.CreatedAt)
		args = append(args, cur.ID)
		conds = append(conds, fmt.Sprintf("(created_at, id) %s ($%d::timestamptz, $%d::uuid)", cmp, len(args)-1, len(args)))
	}

	query := `SELECT ` + tenantColumns + ` FROM tenants`
	if len(conds) > 0 {
		query += " WHERE " + join(conds, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY created_at %s, id %s LIMIT %d", dir, dir, f.Limit+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*domain.Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, "", err
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(tenants) > f.Limit {
		last := tenants[f.Limit-1]
		tenants = tenants[:f.Limit]
		next = encodeCursor(last)
	}
	return tenants, next, nil
}

type scannable interface{ Scan(dest ...any) error }

func scanTenant(row scannable) (*domain.Tenant, error) {
	var (
		id, name, slug, plan, status string
		maxServices                  int
		version                      int64
		createdAt, updatedAt         time.Time
	)
	if err := row.Scan(&id, &name, &slug, &plan, &status, &maxServices, &version, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTenantNotFound
		}
		return nil, fmt.Errorf("scan tenant: %w", err)
	}
	tid, err := domain.TenantIDFromString(id)
	if err != nil {
		return nil, corrupt(err)
	}
	tname, err := domain.NewTenantName(name)
	if err != nil {
		return nil, corrupt(err)
	}
	tslug, err := domain.NewSlug(slug)
	if err != nil {
		return nil, corrupt(err)
	}
	tplan, err := domain.ParsePlan(plan)
	if err != nil {
		return nil, corrupt(err)
	}
	tstatus, err := domain.ParseStatus(status)
	if err != nil {
		return nil, corrupt(err)
	}
	tquota, err := domain.NewQuota(maxServices)
	if err != nil {
		return nil, corrupt(err)
	}
	return domain.Reconstitute(tid, tname, tslug, tplan, tstatus, tquota, version, createdAt, updatedAt), nil
}

func corrupt(err error) error {
	return apperr.Internal("DATA_CORRUPTION", "stored tenant data is invalid").WithCause(err)
}

type cursor struct {
	CreatedAt string `json:"c"`
	ID        string `json:"id"`
	Order     string `json:"o"`
}

func encodeCursor(t *domain.Tenant) string {
	c := cursor{CreatedAt: t.CreatedAt().UTC().Format(time.RFC3339Nano), ID: t.ID().String()}
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(encoded string, order app.SortOrder) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return cursor{}, apperr.Validation("INVALID_CURSOR", "cursor is malformed")
	}
	var c cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return cursor{}, apperr.Validation("INVALID_CURSOR", "cursor is malformed")
	}
	return c, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
