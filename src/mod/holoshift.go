package mod

import (
	"context"
)

type HoloShift struct{}

const HoloShiftModName = "HoloShift"

func (h HoloShift) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, HoloShiftModName, false)
}

func (h HoloShift) EnabledByDefault() bool {
	return false
}
