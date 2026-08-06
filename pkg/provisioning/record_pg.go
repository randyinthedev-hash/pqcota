package provisioning

import (
	"context"
	"strings"

	provisioningv1 "github.com/pqcota/pqcota/gen/pqcota/provisioning/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

const recordSchemaSQL = `
CREATE TABLE IF NOT EXISTS pqcota_provisioning_record (
    seq        BIGSERIAL PRIMARY KEY,
    id         TEXT NOT NULL,
    node_id    TEXT NOT NULL,
    app_keys   TEXT,
    plan_id    TEXT,
    record     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pqcota_prov_node ON pqcota_provisioning_record(node_id, seq);
`

// PgRecordStore — Postgres append-only 프로비저닝 레코드(§1.3 불변·§6A 롤백 근거 영속).
type PgRecordStore struct{ pool *pgxpool.Pool }

func NewPgRecordStore(ctx context.Context, dsn string) (*PgRecordStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, recordSchemaSQL); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgRecordStore{pool: pool}, nil
}

func (p *PgRecordStore) Close() { p.pool.Close() }

func (p *PgRecordStore) Append(r *provisioningv1.ProvisioningRecord) error {
	j, err := protojson.Marshal(r)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(context.Background(),
		`INSERT INTO pqcota_provisioning_record(id,node_id,app_keys,plan_id,record) VALUES($1,$2,$3,$4,$5)`,
		r.GetId(), r.GetNodeId(), strings.Join(r.GetAppKeys(), ","), r.GetPlanId(), j)
	return err
}

func (p *PgRecordStore) All() ([]*provisioningv1.ProvisioningRecord, error) {
	return p.query(`SELECT record FROM pqcota_provisioning_record ORDER BY seq ASC`)
}

func (p *PgRecordStore) ByNode(nodeID string) ([]*provisioningv1.ProvisioningRecord, error) {
	return p.query(`SELECT record FROM pqcota_provisioning_record WHERE node_id=$1 ORDER BY seq ASC`, nodeID)
}

func (p *PgRecordStore) query(sql string, args ...any) ([]*provisioningv1.ProvisioningRecord, error) {
	rows, err := p.pool.Query(context.Background(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*provisioningv1.ProvisioningRecord
	for rows.Next() {
		var j []byte
		if err := rows.Scan(&j); err != nil {
			return nil, err
		}
		r := &provisioningv1.ProvisioningRecord{}
		if err := protojson.Unmarshal(j, r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
