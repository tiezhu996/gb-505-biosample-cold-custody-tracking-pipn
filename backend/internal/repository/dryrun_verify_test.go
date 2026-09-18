package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/model"
)

type fakeDriver struct{}

type fakeConn struct{}

type fakeTx struct{}

func (fakeDriver) Open(string) (driver.Conn, error)  { return fakeConn{}, nil }
func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("dry run") }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return fakeTx{}, nil }
func (fakeTx) Commit() error                         { return nil }
func (fakeTx) Rollback() error                       { return nil }

func init() {
	sql.Register("dryrunpg", fakeDriver{})
}

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := sql.Open("dryrunpg", "")
	if err != nil {
		t.Fatalf("open fake conn: %v", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn, WithoutQuotingCheck: true, PreferSimpleProtocol: true}), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry run db: %v", err)
	}
	return db
}

func TestDryRunReservationSQL(t *testing.T) {
	db := dryRunDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	stmt := db.WithContext(ctx).Model(&model.SlotReservation{}).
		Where("state = ? AND expires_at <= ?", constants.ReservationActive, now).
		Updates(map[string]any{"state": constants.ReservationExpired, "released_at": now, "release_reason": "expired"})
	if stmt.Error != nil {
		t.Fatalf("expire sweep: %v", stmt.Error)
	}
	t.Logf("expire sweep SQL: %s", stmt.Statement.SQL.String())

	var reserved int64
	stmt = db.WithContext(ctx).Model(&model.SlotReservation{}).
		Where("container_id = ? AND position = ? AND state = ? AND expires_at > ?", 2, "R01", constants.ReservationActive, now).
		Count(&reserved)
	if stmt.Error != nil {
		t.Fatalf("slot reserved count: %v", stmt.Error)
	}
	t.Logf("slot reserved count SQL: %s", stmt.Statement.SQL.String())

	containerID := uint(2)
	items := []model.CustodyTransfer{{
		TransferNo: "CT-1", State: constants.TransferStatePrepared,
		ToContainerID: &containerID, ToPosition: "R01", PreparedAt: now,
	}}
	repo := &transferRepository{db: db}
	if _, err := repo.SlotConflicts(ctx, items); err != nil {
		t.Fatalf("slot conflicts dry run: %v", err)
	}

	reservation := model.SlotReservation{TransferID: 1, ContainerID: 2, Position: "R01", State: constants.ReservationActive, ExpiresAt: now}
	stmt = db.WithContext(ctx).Create(&reservation)
	if stmt.Error != nil {
		t.Fatalf("insert reservation: %v", stmt.Error)
	}
	t.Logf("insert reservation SQL: %s", stmt.Statement.SQL.String())
}
