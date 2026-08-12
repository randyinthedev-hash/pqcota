package history

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutoDDLEnv — 이 환경변수가 "0"이면 저장소를 열 때 스키마를 만들지 않는다.
//
// 생성자가 말없이 `CREATE TABLE`을 도는 것은 혼자 쓸 때 편하다. 그런데 가리키는 곳이 어긋나면
// **에러 없이 빈 테이블이 새로 생기고 거기에 쓴다** — 데이터가 사라진 것처럼 보이고, 여러
// 조직을 담는 저장소라면 남의 자리에 쓴다. 스키마 배포를 의도적 행위로 만들어야 하는 배포는
// 이것을 끄고, 스키마는 마이그레이션으로 미리 올린다.
//
// 끈 상태에서 스키마가 없으면 [ErrSchemaMissing]으로 끊는다 — 조용히 만들어 주지 않는다.
const AutoDDLEnv = "PQCOTA_AUTO_DDL"

// ErrSchemaMissing — 자동 DDL이 꺼져 있는데 테이블이 없다.
var ErrSchemaMissing = errors.New("인벤토리 스키마가 없다")

// autoDDL — 기본은 켬. 명시적으로 "0"일 때만 끈다.
func autoDDL() bool { return os.Getenv(AutoDDLEnv) != "0" }

// ensureSchema — 스키마를 올리거나, 올리지 않기로 했으면 있는지 확인만 한다.
func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if autoDDL() {
		_, err := pool.Exec(ctx, schemaSQL)
		return err
	}
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass('pqcota_snapshots') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %s=0으로 자동 생성을 껐다 — 스키마를 먼저 올리거나 search_path를 확인할 것",
			ErrSchemaMissing, AutoDDLEnv)
	}
	return nil
}
