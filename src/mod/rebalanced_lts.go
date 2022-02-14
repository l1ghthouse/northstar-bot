package mod

import (
	"context"
	"fmt"
	"strings"
)

type RebalancedLTS struct{}

const LTSRebalancedRepoOwner = "Dinorush"
const LTSRebalancedRepoName = "LTSRebalance"
const LTSRebalancedModName = LTSRebalancedRepoOwner + "." + LTSRebalancedRepoName
const LTSRebalancedModNameKVFix = LTSRebalancedModName + "_KVFix"

func (h RebalancedLTS) ModParams(ctx context.Context) (string, string, string, string, bool, error) {
	latestTag, err := latestGithubTag(ctx, LTSRebalancedRepoOwner, LTSRebalancedRepoName)
	if err != nil {
		return "", "", "", "", false, err
	}
	link := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s_%s.zip", LTSRebalancedRepoOwner, LTSRebalancedRepoName, latestTag, LTSRebalancedModName, latestTag)
	builder := strings.Builder{}
	builder.WriteString(cmdWgetZipBuilder(link, LTSRebalancedModName))
	builder.WriteString(cmdUnzipBuilderWithDst(LTSRebalancedModName))
	dockerArgs := dockerArgBuilder(fmt.Sprintf("/%s/%s", LTSRebalancedModName, LTSRebalancedModName), LTSRebalancedModName) + " " +
		dockerArgBuilder(fmt.Sprintf("/%s/%s", LTSRebalancedModName, LTSRebalancedModNameKVFix), LTSRebalancedModNameKVFix)
	return builder.String(), dockerArgs, link, latestTag, true, nil
}
