package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	"github.com/jackc/pgx/v4/pgxpool"
	logrus "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type PostgresStore struct{}

func (s *PostgresStore) Initialize() (func(), error) {
	uri := config.DbUri
	maxRetries := 30
	retryInterval := 2 * time.Second

	pgxpoolConfig, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.ConnectConfig(context.Background(), pgxpoolConfig)
		if err == nil {
			break
		}
		if attempt == maxRetries {
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}
		logrus.WithError(err).Warnf("Database connection attempt %d/%d failed, retrying in %v", attempt, maxRetries, retryInterval)
		time.Sleep(retryInterval)
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err = gorm.Open(postgres.Open(uri), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("opening gorm connection: %w", err)
	}

	return func() { pool.Close() }, nil
}

// House operations

func (s *PostgresStore) CreateHouse(ctx context.Context, house *models.House) error {
	return db.WithContext(ctx).Create(house).Error
}

func (s *PostgresStore) GetHouseByID(ctx context.Context, houseID string) (*models.House, error) {
	var house models.House
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).First(&house).Error; err != nil {
		return nil, err
	}
	return &house, nil
}

func (s *PostgresStore) UpdateHouse(ctx context.Context, house *models.House) error {
	return db.WithContext(ctx).Save(house).Error
}

func (s *PostgresStore) DeleteHouse(ctx context.Context, houseID string) error {
	return db.WithContext(ctx).Where("house_id = ?", houseID).Delete(&models.House{}).Error
}

func (s *PostgresStore) ListHouses(ctx context.Context, limit, offset int) ([]models.House, error) {
	var houses []models.House
	if err := db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&houses).Error; err != nil {
		return nil, err
	}
	return houses, nil
}

// Member operations

func (s *PostgresStore) CreateMember(ctx context.Context, member *models.Member) error {
	return db.WithContext(ctx).Create(member).Error
}

func (s *PostgresStore) GetMemberByID(ctx context.Context, memberID string) (*models.Member, error) {
	var member models.Member
	if err := db.WithContext(ctx).Where("member_id = ?", memberID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *PostgresStore) GetMemberByIdentity(ctx context.Context, houseID, domain, userID string) (*models.Member, error) {
	var member models.Member
	if err := db.WithContext(ctx).Where("house_id = ? AND linkkeys_domain = ? AND linkkeys_user_id = ?", houseID, domain, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *PostgresStore) UpdateMember(ctx context.Context, member *models.Member) error {
	return db.WithContext(ctx).Save(member).Error
}

func (s *PostgresStore) DeleteMember(ctx context.Context, memberID string) error {
	return db.WithContext(ctx).Where("member_id = ?", memberID).Delete(&models.Member{}).Error
}

func (s *PostgresStore) ListMembersByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Member, error) {
	var members []models.Member
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("display_name ASC").Limit(limit).Offset(offset).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// Trusted domain operations

func (s *PostgresStore) CreateTrustedDomain(ctx context.Context, td *models.TrustedDomain) error {
	return db.WithContext(ctx).Create(td).Error
}

func (s *PostgresStore) DeleteTrustedDomain(ctx context.Context, tdID string) error {
	return db.WithContext(ctx).Where("trusted_domain_id = ?", tdID).Delete(&models.TrustedDomain{}).Error
}

func (s *PostgresStore) ListTrustedDomains(ctx context.Context, houseID string) ([]models.TrustedDomain, error) {
	var domains []models.TrustedDomain
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("domain ASC").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

func (s *PostgresStore) IsDomainTrusted(ctx context.Context, houseID, domain string) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&models.TrustedDomain{}).Where("house_id = ? AND domain = ?", houseID, domain).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Event operations

func (s *PostgresStore) CreateEvent(ctx context.Context, event *models.Event) error {
	return db.WithContext(ctx).Create(event).Error
}

func (s *PostgresStore) GetEventByID(ctx context.Context, eventID string) (*models.Event, error) {
	var event models.Event
	if err := db.WithContext(ctx).Where("event_id = ?", eventID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *PostgresStore) UpdateEvent(ctx context.Context, event *models.Event) error {
	return db.WithContext(ctx).Save(event).Error
}

func (s *PostgresStore) DeleteEvent(ctx context.Context, eventID string) error {
	return db.WithContext(ctx).Where("event_id = ?", eventID).Delete(&models.Event{}).Error
}

func (s *PostgresStore) ListEventsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Event, error) {
	var events []models.Event
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("starts_at ASC NULLS LAST").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// Task operations

func (s *PostgresStore) CreateTask(ctx context.Context, task *models.Task) error {
	return db.WithContext(ctx).Create(task).Error
}

func (s *PostgresStore) GetTaskByID(ctx context.Context, taskID string) (*models.Task, error) {
	var task models.Task
	if err := db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *PostgresStore) UpdateTask(ctx context.Context, task *models.Task) error {
	return db.WithContext(ctx).Save(task).Error
}

func (s *PostgresStore) DeleteTask(ctx context.Context, taskID string) error {
	return db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&models.Task{}).Error
}

func (s *PostgresStore) ListTasksByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Task, error) {
	var tasks []models.Task
	if err := db.WithContext(ctx).Where("house_id = ?", houseID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// Comment operations

func (s *PostgresStore) CreateComment(ctx context.Context, comment *models.Comment) error {
	return db.WithContext(ctx).Create(comment).Error
}

func (s *PostgresStore) GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error) {
	var comment models.Comment
	if err := db.WithContext(ctx).Where("comment_id = ?", commentID).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *PostgresStore) UpdateComment(ctx context.Context, comment *models.Comment) error {
	return db.WithContext(ctx).Save(comment).Error
}

func (s *PostgresStore) DeleteComment(ctx context.Context, commentID string) error {
	return db.WithContext(ctx).Where("comment_id = ?", commentID).Delete(&models.Comment{}).Error
}

func (s *PostgresStore) ListCommentsByTarget(ctx context.Context, targetType, targetID string, limit, offset int) ([]models.Comment, error) {
	var comments []models.Comment
	if err := db.WithContext(ctx).Where("target_type = ? AND target_id = ?", targetType, targetID).Order("created_at ASC").Limit(limit).Offset(offset).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

// Share operations

func (s *PostgresStore) CreateShare(ctx context.Context, share *models.Share) error {
	return db.WithContext(ctx).Create(share).Error
}

func (s *PostgresStore) DeleteShare(ctx context.Context, shareID string) error {
	return db.WithContext(ctx).Where("share_id = ?", shareID).Delete(&models.Share{}).Error
}

func (s *PostgresStore) ListSharesByResource(ctx context.Context, resourceType, resourceID string) ([]models.Share, error) {
	var shares []models.Share
	if err := db.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (s *PostgresStore) GetShareAccess(ctx context.Context, domain, userID, resourceType, resourceID string) (*models.Share, error) {
	var share models.Share
	if err := db.WithContext(ctx).Where(
		"linkkeys_domain = ? AND linkkeys_user_id = ? AND resource_type = ? AND resource_id = ? AND (expires_at IS NULL OR expires_at > NOW())",
		domain, userID, resourceType, resourceID,
	).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}
