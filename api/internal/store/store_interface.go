package store

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

var AppStore Store

type Store interface {
	Initialize() (cleanup func(), err error)

	// House operations
	CreateHouse(ctx context.Context, house *models.House) error
	GetHouseByID(ctx context.Context, houseID string) (*models.House, error)
	UpdateHouse(ctx context.Context, house *models.House) error
	DeleteHouse(ctx context.Context, houseID string) error
	ListHouses(ctx context.Context, limit, offset int) ([]models.House, error)

	// Member operations
	CreateMember(ctx context.Context, member *models.Member) error
	GetMemberByID(ctx context.Context, memberID string) (*models.Member, error)
	GetMemberByIdentity(ctx context.Context, houseID, domain, userID string) (*models.Member, error)
	UpdateMember(ctx context.Context, member *models.Member) error
	DeleteMember(ctx context.Context, memberID string) error
	ListMembersByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Member, error)

	// Trusted domain operations
	CreateTrustedDomain(ctx context.Context, td *models.TrustedDomain) error
	DeleteTrustedDomain(ctx context.Context, tdID string) error
	ListTrustedDomains(ctx context.Context, houseID string) ([]models.TrustedDomain, error)
	IsDomainTrusted(ctx context.Context, houseID, domain string) (bool, error)

	// Event operations
	CreateEvent(ctx context.Context, event *models.Event) error
	GetEventByID(ctx context.Context, eventID string) (*models.Event, error)
	UpdateEvent(ctx context.Context, event *models.Event) error
	DeleteEvent(ctx context.Context, eventID string) error
	ListEventsByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Event, error)

	// Task operations
	CreateTask(ctx context.Context, task *models.Task) error
	GetTaskByID(ctx context.Context, taskID string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, taskID string) error
	ListTasksByHouse(ctx context.Context, houseID string, limit, offset int) ([]models.Task, error)

	// Comment operations
	CreateComment(ctx context.Context, comment *models.Comment) error
	GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error)
	UpdateComment(ctx context.Context, comment *models.Comment) error
	DeleteComment(ctx context.Context, commentID string) error
	ListCommentsByTarget(ctx context.Context, targetType, targetID string, limit, offset int) ([]models.Comment, error)

	// Share operations
	CreateShare(ctx context.Context, share *models.Share) error
	DeleteShare(ctx context.Context, shareID string) error
	ListSharesByResource(ctx context.Context, resourceType, resourceID string) ([]models.Share, error)
	GetShareAccess(ctx context.Context, domain, userID, resourceType, resourceID string) (*models.Share, error)
}
