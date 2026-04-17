package daemon

import (
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// getRequesterID extracts the requester ID from the proto RequesterInfo message.
// Returns the requester_id field, or "unknown" if not provided.
func getRequesterID(requesterInfo *v1.RequesterInfo) string {
	if requesterInfo == nil || requesterInfo.RequesterId == "" {
		return "unknown"
	}
	return requesterInfo.RequesterId
}
