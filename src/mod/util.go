package mod

import (
	"context"
	"fmt"

	"github.com/google/go-github/v42/github"
)

func latestTag(ctx context.Context, repoOwner string, repoName string) (string, error) {
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
