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

// SeedInitialAdmin creates a default house, the canonical admin/member
// roles for that house, and an admin member from the
// LONGHOUSE_INITIAL_ADMIN_* config — but only if the members table is
// empty. The linkkeys_user_id must be a valid UUID; anything else is
// refused so we never persist an identity that linkkeys cannot assert.
// No-op once any member exists.
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

	adminRole := &models.Role{HouseID: house.HouseID, Name: models.RoleAdmin, Description: "Full administrative access"}
	if err := store.AppStore.CreateRole(ctx, adminRole); err != nil {
		return fmt.Errorf("creating admin role: %w", err)
	}
	memberRole := &models.Role{HouseID: house.HouseID, Name: models.RoleMember, Description: "Standard member"}
	if err := store.AppStore.CreateRole(ctx, memberRole); err != nil {
		return fmt.Errorf("creating member role: %w", err)
	}

	member := &models.Member{
		HouseID:        house.HouseID,
		LinkkeysDomain: domain,
		LinkkeysUserID: userID,
	}
	if err := store.AppStore.CreateMember(ctx, member); err != nil {
		return fmt.Errorf("creating initial admin member: %w", err)
	}

	for _, r := range []*models.Role{adminRole, memberRole} {
		if err := store.AppStore.AssignRole(ctx, member.MemberID, r.RoleID); err != nil {
			return fmt.Errorf("assigning %q role: %w", r.Name, err)
		}
		audit := &models.MemberAudit{
			HouseID:         house.HouseID,
			SubjectMemberID: member.MemberID,
			Action:          models.AuditActionRoleGranted,
			TargetType:      strPtr("role"),
			TargetID:        &r.RoleID,
			Detail:          models.JSONMap{"role_name": r.Name, "via": "initial_admin_bootstrap"},
		}
		if err := store.AppStore.RecordMemberAudit(ctx, audit); err != nil {
			return fmt.Errorf("recording audit for %q role grant: %w", r.Name, err)
		}
	}

	// Seed the admin's linkkeys domain as a trusted domain on the founding
	// house so additional identities from the same domain auto-join on
	// their first sign-in. After bootstrap, admins manage the list via
	// the SPA (TrustedDomainService) — this is the only env-var path.
	td := &models.TrustedDomain{HouseID: house.HouseID, Domain: domain}
	if err := store.AppStore.CreateTrustedDomain(ctx, td); err != nil {
		// Non-fatal: the bootstrap admin still works without it; just no
		// auto-provision until an admin adds the row by hand.
		log.WithError(err).WithField("domain", domain).Warn("seed: could not insert initial trusted_domain row")
	} else {
		_ = store.AppStore.RecordMemberAudit(ctx, &models.MemberAudit{
			HouseID:         house.HouseID,
			SubjectMemberID: member.MemberID,
			ActorMemberID:   &member.MemberID,
			Action:          models.AuditActionTrustedDomainAdded,
			TargetType:      strPtr("trusted_domain"),
			TargetID:        &td.TrustedDomainID,
			Detail:          models.JSONMap{"domain": domain, "via": "initial_admin_bootstrap"},
		})
	}

	log.WithFields(log.Fields{
		"house_id":         house.HouseID,
		"house_name":       house.Name,
		"linkkeys_domain":  domain,
		"linkkeys_user_id": userID,
	}).Info("Bootstrapped initial admin")
	return nil
}

// EnsureInitialTrustedDomain backfills the bootstrap admin's linkkeys
// domain into trusted_domains for the house(s) that admin administers.
//
// SeedInitialAdmin already seeds this row on a fresh bootstrap, but it is
// a no-op once any member exists, so a house that was bootstrapped before
// the trusted-domain seeding existed (or where that non-fatal insert
// failed once) would never auto-provision same-domain members — they can
// authenticate but land with an empty house list. This runs every boot
// and is idempotent: it only inserts a row that is genuinely missing, and
// only on houses where the configured admin actually holds the admin role
// (so we never auto-trust a whole domain on a house the admin merely
// joined as a regular member).
func EnsureInitialTrustedDomain() error {
	ctx := context.Background()

	domain := config.InitialAdminDomain
	userID := config.InitialAdminUserID
	if domain == "" || userID == "" {
		return nil
	}

	members, err := store.AppStore.FindMembersByLinkkeysIdentity(ctx, domain, userID)
	if err != nil {
		return fmt.Errorf("looking up bootstrap admin members: %w", err)
	}

	for _, m := range members {
		isAdmin, err := memberHasRole(ctx, m.MemberID, models.RoleAdmin)
		if err != nil {
			return fmt.Errorf("checking admin role for member %s: %w", m.MemberID, err)
		}
		if !isAdmin {
			continue
		}

		trusted, err := store.AppStore.ListTrustedDomains(ctx, m.HouseID)
		if err != nil {
			return fmt.Errorf("listing trusted domains for house %s: %w", m.HouseID, err)
		}
		if containsDomain(trusted, domain) {
			continue
		}

		td := &models.TrustedDomain{HouseID: m.HouseID, Domain: domain}
		if err := store.AppStore.CreateTrustedDomain(ctx, td); err != nil {
			// Non-fatal: don't block boot over one house's backfill.
			log.WithError(err).WithFields(log.Fields{
				"house_id": m.HouseID,
				"domain":   domain,
			}).Warn("backfill: could not insert trusted_domain row")
			continue
		}
		memberID := m.MemberID
		_ = store.AppStore.RecordMemberAudit(ctx, &models.MemberAudit{
			HouseID:         m.HouseID,
			SubjectMemberID: m.MemberID,
			ActorMemberID:   &memberID,
			Action:          models.AuditActionTrustedDomainAdded,
			TargetType:      strPtr("trusted_domain"),
			TargetID:        &td.TrustedDomainID,
			Detail:          models.JSONMap{"domain": domain, "via": "trusted_domain_backfill"},
		})
		log.WithFields(log.Fields{
			"house_id": m.HouseID,
			"domain":   domain,
		}).Info("Backfilled trusted_domain for bootstrap admin's house")
	}

	return nil
}

func memberHasRole(ctx context.Context, memberID, roleName string) (bool, error) {
	roles, err := store.AppStore.ListRolesForMember(ctx, memberID)
	if err != nil {
		return false, err
	}
	for _, r := range roles {
		if r.Name == roleName {
			return true, nil
		}
	}
	return false, nil
}

func containsDomain(tds []models.TrustedDomain, domain string) bool {
	for _, td := range tds {
		if td.Domain == domain {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }
