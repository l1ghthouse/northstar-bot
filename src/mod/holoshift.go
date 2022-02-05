package mod

import (
	"context"
	"fmt"
	"strings"
)

type HoloShift struct{}

const HoloShiftRepoOwner = "Legonzaur"
const HoloShiftRepoName = "Legonzaur.HoloShift"

func (h HoloShift) DockerArg() string {
	return fmt.Sprintf("--mount \"type=bind,source=/%s,target=/mnt/mods/%s,readonly\"", HoloShiftRepoName, HoloShiftRepoName)
}

func (h HoloShift) LatestVersion(ctx context.Context) (string, error) {
	return latestTag(ctx, HoloShiftRepoOwner, HoloShiftRepoName)
}

func (h HoloShift) DownloadLatestVersionURI(ctx context.Context) (string, error) {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/latest/%s.zip", HoloShiftRepoOwner, HoloShiftRepoName, HoloShiftRepoName), nil
}

func (h HoloShift) ModParams(ctx context.Context) (string, string, string, string, error) {
	link, err := h.DownloadLatestVersionURI(ctx)
	if err != nil {
		return "", "", "", "", err
	}

	tag, err := h.LatestVersion(ctx)
	if err != nil {
		return "", "", "", "", err
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("wget %s", link))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("unzip %s.zip -d /", HoloShiftRepoName))
	builder.WriteString("\n")
	return builder.String(), h.DockerArg(), link, tag, nil
}
