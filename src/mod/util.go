package mod

import (
	"context"
	"fmt"
	"github.com/l1ghthouse/northstar-bootstrap/src/mod/thunderstore"
	"strings"

	"github.com/google/go-github/v42/github"
)

func latestGithubTag(ctx context.Context, repoOwner string, repoName string) (string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(ctx, repoOwner, repoName, nil)
	if err != nil {
		return "", fmt.Errorf("error listing tags: %w", err)
	}
	if len(tags) > 0 {
		return tags[0].GetName(), nil
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

func dockerArgBuilder(ModPath string, modName string) string {
	return fmt.Sprintf("--mount \"type=bind,source=%s,target=/mnt/mods/%s,readonly\"", ModPath, modName)
}

func latestThunderstoreMod(ctx context.Context, packageName string, requiredByClient bool) (string, string, string, string, bool, error) {
	pkg, err := thunderstore.GetPackageByName(ctx, packageName)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("failed to get package: %w", err)
	}
	latestVersion, err := thunderstore.GetLatestPackageVersion(pkg)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("failed to get latest package version: %w", err)
	}

	builder := strings.Builder{}
	builder.WriteString(cmdWgetZipBuilder(latestVersion.DownloadURL, packageName))
	builder.WriteString(cmdUnzipBuilderWithDst(packageName))

	modFullName := pkg.Owner + "." + packageName

	return builder.String(), dockerArgBuilder(fmt.Sprintf("/%s/mods/%s", packageName, modFullName), modFullName), latestVersion.DownloadURL, latestVersion.VersionNumber, requiredByClient, nil
}
