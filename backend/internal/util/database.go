package util

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"biosample-cold-custody-tracking/backend/internal/constants"
	"biosample-cold-custody-tracking/backend/internal/model"
)

func OpenDatabase(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access postgres connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(8)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func OpenObjectStore(endpoint, accessKey, secretKey string, useSSL bool) (*minio.Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	return client, nil
}

func EnsureObjectStore(ctx context.Context, client *minio.Client, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check MinIO bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create MinIO bucket: %w", err)
	}
	return nil
}

func ObjectStoreReady(ctx context.Context, client *minio.Client, bucket string) error {
	if client == nil {
		return errors.New("MinIO client is not configured")
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("query MinIO bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("MinIO bucket %q does not exist", bucket)
	}
	return nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.StorageContainer{},
		&model.Specimen{},
		&model.CustodyTransfer{},
		&model.ProtocolReview{},
		&model.AuditLog{},
	); err != nil {
		return err
	}
	const immutableAuditFunction = `
CREATE OR REPLACE FUNCTION reject_audit_log_mutation()
RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;`
	if err := db.Exec(immutableAuditFunction).Error; err != nil {
		return fmt.Errorf("create immutable audit function: %w", err)
	}
	const immutableAuditTrigger = `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'audit_logs_immutable'
  ) THEN
    CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();
  END IF;
END;
$$;`
	if err := db.Exec(immutableAuditTrigger).Error; err != nil {
		return fmt.Errorf("create immutable audit trigger: %w", err)
	}
	return nil
}

func Ready(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// SeedDemoData is idempotent and only initializes an empty custody catalog.
func SeedDemoData(ctx context.Context, db *gorm.DB) error {
	var specimenCount int64
	if err := db.WithContext(ctx).Model(&model.Specimen{}).Count(&specimenCount).Error; err != nil {
		return fmt.Errorf("count specimens: %w", err)
	}
	if specimenCount > 0 {
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		containers := []model.StorageContainer{
			{
				Code:            "FZ-20-A01",
				Name:            "负二十度一号冻存柜",
				ContainerType:   "mechanical_freezer",
				TemperatureZone: "minus20",
				Location:        "样本库 A 区",
				Capacity:        500,
				Occupied:        1,
				Status:          "available",
				Active:          true,
			},
			{
				Code:            "FZ-80-B02",
				Name:            "负八十度二号冻存柜",
				ContainerType:   "ultra_low_freezer",
				TemperatureZone: "minus80",
				Location:        "样本库 B 区",
				Capacity:        800,
				Occupied:        2,
				Status:          "available",
				Active:          true,
			},
			{
				Code:            "LN2-C03",
				Name:            "液氮三号罐",
				ContainerType:   "vapor_phase_tank",
				TemperatureZone: "liquid_nitrogen",
				Location:        "样本库 C 区",
				Capacity:        1200,
				Occupied:        0,
				Status:          "maintenance",
				Active:          true,
			},
		}
		if err := tx.Create(&containers).Error; err != nil {
			return fmt.Errorf("seed storage containers: %w", err)
		}

		now := time.Now().UTC()
		receivedOne := now.Add(-72 * time.Hour)
		receivedTwo := now.Add(-30 * time.Hour)
		receivedThree := now.Add(-6 * time.Hour)
		receivedFour := now.Add(-2 * time.Hour)
		expiresOne := now.AddDate(2, 0, 0)
		expiresTwo := now.AddDate(1, 6, 0)
		expiresThree := now.AddDate(1, 0, 0)
		specimens := []model.Specimen{
			{
				AccessionNo:        "BIO-20260819-001",
				SampleType:         "血浆",
				SubjectCode:        "SUBJ-A1038",
				ProtocolCode:       "PROTO-ONC-042",
				State:              constants.SpecimenStateStored,
				StorageContainerID: &containers[1].ID,
				Position:           "R02-BX04-A03",
				VolumeML:           4.5,
				AliquotCount:       3,
				CurrentCustodian:   "冻存保管员",
				ReceivedAt:         receivedOne,
				ExpiresAt:          &expiresOne,
				Notes:              "三支等体积分装",
			},
			{
				AccessionNo:        "BIO-20260820-014",
				SampleType:         "组织冻存管",
				SubjectCode:        "SUBJ-B2071",
				ProtocolCode:       "PROTO-NEURO-018",
				State:              constants.SpecimenStateStored,
				StorageContainerID: &containers[1].ID,
				Position:           "R04-BX01-C08",
				VolumeML:           2.0,
				AliquotCount:       1,
				CurrentCustodian:   "冻存保管员",
				ReceivedAt:         receivedTwo,
				ExpiresAt:          &expiresTwo,
			},
			{
				AccessionNo:        "BIO-20260821-027",
				SampleType:         "全血",
				SubjectCode:        "SUBJ-C4412",
				ProtocolCode:       "PROTO-CARDIO-006",
				State:              constants.SpecimenStateStored,
				StorageContainerID: &containers[0].ID,
				Position:           "S03-BX07-D02",
				VolumeML:           8.0,
				AliquotCount:       0,
				CurrentCustodian:   "冻存保管员",
				ReceivedAt:         receivedThree,
				ExpiresAt:          &expiresThree,
			},
			{
				AccessionNo:      "BIO-20260822-004",
				SampleType:       "血清",
				SubjectCode:      "SUBJ-D5520",
				ProtocolCode:     "PROTO-IMMUNE-011",
				State:            constants.SpecimenStateReceived,
				VolumeML:         6.0,
				AliquotCount:     0,
				CurrentCustodian: "样本接收员",
				ReceivedAt:       receivedFour,
				Notes:            "等待离心分装",
			},
		}
		if err := tx.Create(&specimens).Error; err != nil {
			return fmt.Errorf("seed specimens: %w", err)
		}

		acceptedAt := now.Add(-28 * time.Hour)
		acceptedBy := uint(1)
		minus80 := -78.4
		transfers := []model.CustodyTransfer{
			{
				SpecimenID:     specimens[1].ID,
				TransferNo:     "CT-20260820-014",
				FromCustodian:  "样本接收员",
				ToCustodian:    "冻存保管员",
				FromLocation:   "intake",
				ToLocation:     "样本库 B 区",
				ToContainerID:  &containers[1].ID,
				ToPosition:     "R04-BX01-C08",
				State:          constants.TransferStateAccepted,
				PreparedByID:   1,
				PreparedByName: "系统初始化",
				AcceptedByID:   &acceptedBy,
				AcceptedByName: "系统初始化",
				PreparedAt:     now.Add(-29 * time.Hour),
				ResolvedAt:     &acceptedAt,
				TemperatureC:   &minus80,
				Reason:         "接收后完成冻存交接",
			},
			{
				SpecimenID:     specimens[3].ID,
				TransferNo:     "CT-20260822-004",
				FromCustodian:  "样本接收员",
				ToCustodian:    "冻存保管员",
				FromLocation:   "intake",
				ToLocation:     "样本库 B 区",
				State:          constants.TransferStatePrepared,
				PreparedByID:   1,
				PreparedByName: "系统初始化",
				PreparedAt:     now.Add(-30 * time.Minute),
				Reason:         "待分装完成后转入负八十度冻存",
			},
		}
		if err := tx.Create(&transfers).Error; err != nil {
			return fmt.Errorf("seed custody transfers: %w", err)
		}

		retention := now.AddDate(5, 0, 0)
		reviews := []model.ProtocolReview{
			{
				SpecimenID:      specimens[0].ID,
				ProtocolCode:    specimens[0].ProtocolCode,
				Decision:        constants.DecisionHold,
				ReviewerID:      1,
				ReviewerName:    "系统初始化",
				ConsentVerified: true,
				ScopeVerified:   true,
				RetentionUntil:  &retention,
				Notes:           "等待课题组补充二次使用范围说明",
				ReviewedAt:      now.Add(-20 * time.Hour),
			},
		}
		if err := tx.Create(&reviews).Error; err != nil {
			return fmt.Errorf("seed protocol reviews: %w", err)
		}
		return nil
	})
}

func IsUniqueViolation(err error) bool {
	return err != nil && errors.Is(err, gorm.ErrDuplicatedKey)
}
