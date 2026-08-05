package inventory

import (
	"context"
	"errors"

	inventoryv1 "github.com/pqcota/pqcota/gen/pqcota/inventory/v1"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

const metaSchemaSQL = `
CREATE TABLE IF NOT EXISTS pqcota_endpoint (
    node_id    TEXT PRIMARY KEY,
    endpoint   JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS pqcota_profile (
    node_id    TEXT PRIMARY KEY,
    profile    JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// PgMetaStore — Postgres 머신 메타데이터. node_id PK로 **upsert**한다(사용자 재사용·수정 가능·§2.0).
// 히스토리(append-only)와 다르다. 접근 비밀은 스키마에 없다 — MachineEndpoint 자체가 비밀 필드가 없다.
type PgMetaStore struct{ pool *pgxpool.Pool }

func NewPgMetaStore(ctx context.Context, dsn string) (*PgMetaStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, metaSchemaSQL); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgMetaStore{pool: pool}, nil
}

func (p *PgMetaStore) Close() { p.pool.Close() }

func (p *PgMetaStore) UpsertEndpoint(e *inventoryv1.MachineEndpoint) error {
	j, err := protojson.Marshal(e)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(context.Background(),
		`INSERT INTO pqcota_endpoint(node_id,endpoint,updated_at) VALUES($1,$2,now())
		 ON CONFLICT(node_id) DO UPDATE SET endpoint=$2, updated_at=now()`, e.GetNodeId(), j)
	return err
}

func (p *PgMetaStore) UpsertProfile(pr *inventoryv1.MachineProfile) error {
	j, err := protojson.Marshal(pr)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(context.Background(),
		`INSERT INTO pqcota_profile(node_id,profile,updated_at) VALUES($1,$2,now())
		 ON CONFLICT(node_id) DO UPDATE SET profile=$2, updated_at=now()`, pr.GetNodeId(), j)
	return err
}

func (p *PgMetaStore) Endpoint(nodeID string) (*inventoryv1.MachineEndpoint, error) {
	var j []byte
	err := p.pool.QueryRow(context.Background(),
		`SELECT endpoint FROM pqcota_endpoint WHERE node_id=$1`, nodeID).Scan(&j)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e := &inventoryv1.MachineEndpoint{}
	return e, protojson.Unmarshal(j, e)
}

func (p *PgMetaStore) Profile(nodeID string) (*inventoryv1.MachineProfile, error) {
	var j []byte
	err := p.pool.QueryRow(context.Background(),
		`SELECT profile FROM pqcota_profile WHERE node_id=$1`, nodeID).Scan(&j)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pr := &inventoryv1.MachineProfile{}
	return pr, protojson.Unmarshal(j, pr)
}
