package mod

import (
	"context"
	"fmt"
	"strings"
)

type RebalancedLTSMod struct{}

const LTSRebalancedRepoOwner = "Dinorush"
const LTSRebalancedRepoName = "LTSRebalance"
const LTSRebalancedModName = LTSRebalancedRepoOwner + "." + LTSRebalancedRepoName

func (h RebalancedLTSMod) DockerArg() string {
	return fmt.Sprintf("--mount \"type=bind,source=/%s,target=/mnt/mods/%s,readonly\"", LTSRebalancedModName, LTSRebalancedModName)
}

func (h RebalancedLTSMod) LatestVersion(ctx context.Context) (string, error) {
	return latestTag(ctx, LTSRebalancedRepoOwner, LTSRebalancedRepoName)
}

func (h RebalancedLTSMod) DownloadLatestVersionURI(ctx context.Context) (string, error) {
	latestTag, err := h.LatestVersion(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s_%s.zip", LTSRebalancedRepoOwner, LTSRebalancedRepoName, latestTag, LTSRebalancedModName, latestTag), nil
}

func (h RebalancedLTSMod) ModParams(ctx context.Context) (string, string, string, string, error) {
	link, err := h.DownloadLatestVersionURI(ctx)
	if err != nil {
		return "", "", "", "", err
	}

	latestTag, err := h.LatestVersion(ctx)
	if err != nil {
		return "", "", "", "", err
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("wget %s", link))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("unzip %s_%s.zip -d /", LTSRebalancedModName, latestTag))
	builder.WriteString("\n")
	return builder.String(), h.DockerArg(), link, latestTag, nil
}
