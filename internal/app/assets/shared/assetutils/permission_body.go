package assetutils

import (
	"strconv"

	"github.com/Obey72/backend/internal/roblox/assets"
)

// newpermissionbodyfromids builds a permissionrequest that grants use access
// to the given universes used by the upload pipeline so the target place's
// universe can use the freshly-uploaded asset
func NewPermissionBodyFromIds(universeIDs []int64) assets.PermissionRequest {
	requests := make([]assets.PermissionRequestItem, len(universeIDs))

	for i, universeID := range universeIDs {
		requests[i] = assets.PermissionRequestItem{
			SubjectType: "Universe",
			SubjectID:   strconv.FormatInt(universeID, 10),
			Action:      "Use",
		}
	}

	return assets.PermissionRequest{
		Requests: requests,
	}
}

// newpermissionbodymulti builds a permissionrequest that grants use access to
// the given universes and a single holder user used in multi-cookie mode where
// the uploader account is different from the place owner: the universe grant
// covers in-place playback the user grant gives the holder admin authority
// over the asset (eg to grant further permissions or move it between places)
//
// holderuserid == 0 means no holder behaves like newpermissionbodyfromids
func NewPermissionBodyMulti(universeIDs []int64, holderUserID int64) assets.PermissionRequest {
	count := len(universeIDs)
	if holderUserID > 0 {
		count++
	}
	requests := make([]assets.PermissionRequestItem, 0, count)

	for _, universeID := range universeIDs {
		requests = append(requests, assets.PermissionRequestItem{
			SubjectType: "Universe",
			SubjectID:   strconv.FormatInt(universeID, 10),
			Action:      "Use",
		})
	}

	if holderUserID > 0 {
		requests = append(requests, assets.PermissionRequestItem{
			SubjectType: "User",
			SubjectID:   strconv.FormatInt(holderUserID, 10),
			Action:      "Use",
		})
	}

	return assets.PermissionRequest{
		Requests: requests,
	}
}
