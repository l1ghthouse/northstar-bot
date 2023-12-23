package mod

import (
	"context"
	"fmt"
)

type TitanDebug struct{}

func (r TitanDebug) Validate(otherMods []Mod) error {
	for _, mod := range otherMods {
		_, ok := mod.(*RebalancedLTS)
		if ok {
			return fmt.Errorf("cannot have both rebalanced, and titan debug enabled. Please explicitly enable/disable both mods")
		}
	}
	return nil
}

func (h TitanDebug) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, "TitanDebug")
}

func (h TitanDebug) EnabledByDefault() bool {
	return false
}
