package mod

import (
	"context"
	"fmt"
)

type Mod interface {
	ModParams(ctx context.Context) (cmd string, dockerArgs string, downloadLink string, version string, requiredByClient bool, err error)
	Validate(otherMods []Mod) error
	EnabledByDefault() bool
}

const RebalancedLtsModTest = "rebalanced_lts_mod_test"

var ByName = map[string]func() Mod{
	"pg9182_metrics": func() Mod {
		return &PG9182Metrics{}
	},
	"rebalanced_lts_mod": func() Mod { return &RebalancedLTS{PreRelease: false} },
	RebalancedLtsModTest: func() Mod { return &RebalancedLTS{PreRelease: true} },
	"titan_debug":        func() Mod { return &TitanDebug{} },
	"ctf_experimental":   func() Mod { return &TestCTFSpawns{} },
	"remove_navmesh":     func() Mod { return &RemoveNavmesh{} },
	"holo_shift_mod": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "HoloShift",
		}
	},
	"parseable_logs": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "ParseableLogs",
		}
	},
	"ramp_water": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "RampWater",
		}
	},
	"better_homestead": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "BetterHomestead",
		}
	},
	"better_rise": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "BetterRise",
		}
	},
	"archon": func() Mod {
		return &ThunderstoreMod{
			Enabled: false,
			Name:    "MoblinArchon",
		}
	},
}

type ThunderstoreMod struct {
	Enabled bool
	Name    string
}

func (r ThunderstoreMod) Validate(otherMods []Mod) error {
	return nil
}

func (h ThunderstoreMod) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	return latestThunderstoreMod(ctx, h.Name)
}

func (h ThunderstoreMod) EnabledByDefault() bool {
	return h.Enabled
}

var ErrNoTagsFound = fmt.Errorf("no tags found")
