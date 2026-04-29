package cmd

import (
	"context"
	"fmt"
	"regexp"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// SeedInitialAdmin creates a default house and an admin member from the
// LONGHOUSE_INITIAL_ADMIN_* config if the members table is empty. The
// linkkeys_user_id must be a valid UUID; anything else is refused so we never
// persist an identity that linkkeys cannot assert. No-op once any member
// exists.
func SeedInitialAdmin() error {
	ctx := context.Background()

	houses, err := store.AppStore.ListHouses(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("checking for existing houses: %w", err)
	}
	if len(houses) > 0 {
		members, err := store.AppStore.ListMembersByHouse(ctx, houses[0].HouseID, 1, 0)
		if err != nil {
			return fmt.Errorf("checking for existing members: %w", err)
		}
		if len(members) > 0 {
			return nil
		}
	}

	domain := config.InitialAdminDomain
	userID := config.InitialAdminUserID
	if domain == "" || userID == "" {
		log.Warn("No members in database and LONGHOUSE_INITIAL_ADMIN_DOMAIN / LONGHOUSE_INITIAL_ADMIN_USER_ID are not set; skipping admin bootstrap")
		return nil
	}
	if !uuidRegex.MatchString(userID) {
		log.WithField("user_id", userID).Warn("LONGHOUSE_INITIAL_ADMIN_USER_ID is not a valid UUID; skipping admin bootstrap")
		return nil
	}

	house := &models.House{Name: config.InitialHouseName}
	if err := store.AppStore.CreateHouse(ctx, house); err != nil {
		return fmt.Errorf("creating initial house: %w", err)
	}

	member := &models.Member{
		HouseID:        house.HouseID,
		LinkkeysDomain: domain,
		LinkkeysUserID: userID,
		Roles:          models.StringList{"admin", "member"},
	}
	if err := store.AppStore.CreateMember(ctx, member); err != nil {
		return fmt.Errorf("creating initial admin member: %w", err)
	}

	log.WithFields(log.Fields{
		"house_id":         house.HouseID,
		"house_name":       house.Name,
		"linkkeys_domain":  domain,
		"linkkeys_user_id": userID,
	}).Info("Bootstrapped initial admin")
	return nil
}
