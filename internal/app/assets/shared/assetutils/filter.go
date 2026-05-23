package assetutils

import (
	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/roblox/develop"
)

func NewFilter(ctx *context.Context, r *request.Request, assetTypeID int32) func(assetsInfo develop.GetAssetsInfoResponse) []*develop.AssetInfo {
	creatorID := r.CreatorID
	userID := ctx.Client.UserInfo.ID
	// when reuploadalreadyowned is on, do not skip assets owned by the running
	// account or the destination creator we still skip roblox-owned (id 1)
	// because those are not reuploadable
	checkUserID := !r.IsGroup && !r.ReuploadAlreadyOwned
	checkCreator := !r.ReuploadAlreadyOwned

	return func(assetsInfo develop.GetAssetsInfoResponse) []*develop.AssetInfo {
		filteredAssetsInfo := assetsInfo.Data[:0]
		for _, info := range assetsInfo.Data {
			if info.TypeID != assetTypeID {
				continue
			}

			assetCreatorID := info.Creator.TargetID
			if assetCreatorID == 1 {
				continue
			}
			if checkCreator && assetCreatorID == creatorID {
				continue
			}
			if checkUserID && assetCreatorID == userID {
				continue
			}

			filteredAssetsInfo = append(filteredAssetsInfo, info)
		}
		return filteredAssetsInfo
	}
}
