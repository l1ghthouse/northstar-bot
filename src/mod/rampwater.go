package mod

import (
	"context"
)

type RampWater struct{}

const RampWaterModName = "RampWater"

func (h RampWater) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, RampWaterModName, false)
}

func (h RampWater) EnabledByDefault() bool {
	return false
}
