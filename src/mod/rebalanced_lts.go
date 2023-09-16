package mod

import (
	"context"
	"fmt"
	"strings"
)

type RebalancedLTS struct {
	PreRelease bool
}

const LTSRebalancedRepoOwner = "Dinorush"
const LTSRebalancedRepoName = "LTSRebalance"
const LTSRebalancedModName = LTSRebalancedRepoOwner + "." + LTSRebalancedRepoName
const LTSRebalancedModNameKVFix = LTSRebalancedModName + "_KVFix"

func (r RebalancedLTS) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	latestTag, err := latestGithubReleaseTag(ctx, LTSRebalancedRepoOwner, LTSRebalancedRepoName, r.PreRelease)
	if err != nil {
		return "", "", "", "", false, err
	}
	link := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s_%s.zip", LTSRebalancedRepoOwner, LTSRebalancedRepoName, latestTag, LTSRebalancedModName, latestTag)
	builder := strings.Builder{}
	builder.WriteString(cmdWgetZipBuilder(link, LTSRebalancedModName))
	builder.WriteString(cmdUnzipBuilderWithDst(LTSRebalancedModName))
	builder.WriteString(fmt.Sprintf("cp -r /%s/mods/* /mods/", LTSRebalancedModName))
	builder.WriteString("\n")
	return builder.String(), "", link, latestTag, true, nil
}

func (r RebalancedLTS) Validate(otherMods []Mod) error {
	for _, mod := range otherMods {
		v, ok := mod.(*RebalancedLTS)
		if ok && v.PreRelease != r.PreRelease {
			return fmt.Errorf("cannot have both pre-release and release versions of Rebalanced LTS enabled. Please explicitly enable/disable both mods")
		}
	}
	return nil
}

func (r RebalancedLTS) EnabledByDefault() bool {
	return false
}
