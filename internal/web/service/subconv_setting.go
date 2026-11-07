package service

import (
	"encoding/json"

	engmodel "github.com/mhsanaei/3x-ui/v3/internal/subconvengine/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// validateSubConvConf rejects a stored mihomo engine config that is neither
// empty (defaults) nor a JSON object the engine can actually load.
func validateSubConvConf(raw string) error {
	if raw == "" {
		return nil
	}
	var opts engmodel.RenderOptions
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return common.NewError("Mihomo engine config", err.Error())
	}
	return nil
}
