package mod

import (
	"context"
	"fmt"
	"strings"

	"github.com/l1ghthouse/northstar-bootstrap/src/mod/thunderstore"

	"github.com/google/go-github/v42/github"
)

func latestGithubReleaseTag(ctx context.Context, repoOwner string, repoName string, preRelease bool) (string, error) {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(ctx, repoOwner, repoName, nil)
	if err != nil {
		return "", fmt.Errorf("error listing tags: %w", err)
	}

	for i := range releases {
		if (preRelease && releases[i].GetPrerelease()) || (!preRelease && !releases[i].GetPrerelease()) {
			return releases[i].GetTagName(), nil
		}
	}
	return "", ErrNoTagsFound
}

func cmdWgetZipBuilder(link string, zipName string) string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("wget %s -O %s.zip", link, zipName))
	builder.WriteString("\n")
	return builder.String()
}

func cmdUnzipBuilder(zipName string) string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("unzip %s.zip -d /", zipName))
	builder.WriteString("\n")
	return builder.String()
}

func cmdUnzipBuilderWithDst(zipName string) string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("mkdir -p /%s", zipName))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("unzip %s.zip -d /%s", zipName, zipName))
	builder.WriteString("\n")
	return builder.String()
}

func latestThunderstoreMod(ctx context.Context, packageName string) (string, string, string, string, bool, error) {
	pkg, err := thunderstore.GetPackageByName(ctx, packageName)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("failed to get package: %w", err)
	}
	latestVersion, err := thunderstore.GetLatestPackageVersion(pkg)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("failed to get latest package version: %w", err)
	}

	builder := strings.Builder{}
	builder.WriteString(cmdWgetZipBuilder(latestVersion.DownloadURL, pkg.Name))
	builder.WriteString(cmdUnzipBuilderWithDst(pkg.Name))
	builder.WriteString(fmt.Sprintf("cp -r /%s/mods/* /mods/", pkg.Name))
	builder.WriteString("\n")

	requiredByClient := false

	for _, category := range pkg.Categories {
		if strings.Contains(category, "Client-side") {
			requiredByClient = true
		}
	}

	return builder.String(), "", latestVersion.DownloadURL, latestVersion.VersionNumber, requiredByClient, nil
}
