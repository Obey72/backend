package assetutils

import "github.com/Obey72/backend/internal/roblox/assetdelivery"

func NewBatchBodyFromIDs(assetIDs []int64) []*assetdelivery.AssetRequestItem {
	body := make([]*assetdelivery.AssetRequestItem, 0)
	for _, id := range assetIDs {
		body = append(body, &assetdelivery.AssetRequestItem{
			AssetID:   id,
			RequestID: "0",
		})
	}
	return body
}
