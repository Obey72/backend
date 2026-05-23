package permissions

import (
	"errors"

	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/roblox"
	"github.com/Obey72/backend/internal/roblox/develop"
	"github.com/Obey72/backend/internal/roblox/groups"
)

var (
	ErrNotMember              = errors.New("account is not in group")
	ErrNoCreateItemPermission = errors.New("account does not have permission to create items for group")
	ErrNoManageGroupGames     = errors.New("account does not have permission to manage group games")
	ErrNoEditPermission       = errors.New("account does not have permission to edit place")
)

func canEditGroup(c *roblox.Client, groupID int64) error {
	groupMembership, err := groups.Membership(c, groupID)
	if err != nil {
		return err
	}

	if groupMembership.UserRole.Role.Name == "Guest" {
		return ErrNotMember
	}

	groupPermissions := groupMembership.Permissions.GroupEconomyPermissions
	if canCreateItems := groupPermissions.CreateItems; !canCreateItems {
		return ErrNoCreateItemPermission
	}

	if canManageGames := groupPermissions.ManageGroupGames; !canManageGames {
		return ErrNoManageGroupGames
	}

	return nil
}

func CanEditUniverse(ctx *context.Context, r *request.Request) error {
	if r.IsGroup {
		return canEditGroup(ctx.Client, r.CreatorID)
	}

	_, err := develop.TeamCreateSettings(ctx.Client, r.UniverseID)
	if err == develop.TeamCreateSettingsErrors.ErrAuthorizationDenied {
		return ErrNoEditPermission
	}

	return err
}
