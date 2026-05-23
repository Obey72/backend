package request

import (
	"github.com/Obey72/backend/internal/roblox"
	"github.com/Obey72/backend/internal/roblox/games"
)

type RawRequest struct {
	PlaceID         int64   `json:"placeId"`
	CreatorID       int64   `json:"creatorId"`
	IDs             []int64 `json:"ids"`
	DefaultPlaceIDs []int64 `json:"defaultPlaceIds"`
	PluginVersion   string  `json:"pluginVersion"`
	AssetType       string  `json:"assetType"`
	ExportJSON      bool    `json:"exportJSON"`
	IsGroup         bool    `json:"isGroup"`
	// holderuserid kept for backwards compatibility with old plugin builds new
	// callers should send holderuserids instead a non-zero value here is merged
	// into holderuserids on the server side
	HolderUserID int64 `json:"holderUserId"`
	// holderuserids is the list of accounts that get use permission granted on
	// each upload from a non-holder account 0 means no holder configured an
	// empty list also means none
	HolderUserIDs []int64 `json:"holderUserIds"`
	// when true the filter does not skip assets the current account or the
	// destination creator already owns useful when the user wants fresh ids
	// even for assets they already uploaded
	ReuploadAlreadyOwned bool `json:"reuploadAlreadyOwned"`
	// when true holder grants run even in single-account mode lets the user
	// share permission on freshly uploaded assets with one or more other
	// accounts without needing a credential pool
	PermissionSharing bool `json:"permissionSharing"`
}

type Request struct {
	UniverseID           int64
	PlaceID              int64
	CreatorID            int64
	IDs                  []int64
	DefaultPlaceIDs      []int64
	IsGroup              bool
	HolderUserIDs        []int64
	ReuploadAlreadyOwned bool
	PermissionSharing    bool
}

func FromRawRequest(c *roblox.Client, req *RawRequest) (*Request, error) {
	placeID := req.PlaceID

	placesInfo, err := games.MultiGetPlaceDetails(c, []int64{placeID})
	if err != nil {
		return nil, err
	}

	holders := make([]int64, 0, len(req.HolderUserIDs)+1)
	seen := make(map[int64]struct{})
	if req.HolderUserID > 0 {
		holders = append(holders, req.HolderUserID)
		seen[req.HolderUserID] = struct{}{}
	}
	for _, id := range req.HolderUserIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		holders = append(holders, id)
	}

	return &Request{
		UniverseID:           placesInfo[0].UniverseID,
		PlaceID:              placeID,
		CreatorID:            req.CreatorID,
		IDs:                  req.IDs,
		DefaultPlaceIDs:      req.DefaultPlaceIDs,
		IsGroup:              req.IsGroup,
		HolderUserIDs:        holders,
		ReuploadAlreadyOwned: req.ReuploadAlreadyOwned,
		PermissionSharing:    req.PermissionSharing,
	}, nil
}
