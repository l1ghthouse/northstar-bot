package thunderstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Package struct {
	Name           string    `json:"name"`
	FullName       string    `json:"full_name"`
	Owner          string    `json:"owner"`
	PackageURL     string    `json:"package_url"`
	DateCreated    time.Time `json:"date_created"`
	DateUpdated    time.Time `json:"date_updated"`
	UUID4          string    `json:"uuid4"`
	RatingScore    int       `json:"rating_score"`
	IsPinned       bool      `json:"is_pinned"`
	IsDeprecated   bool      `json:"is_deprecated"`
	HasNsfwContent bool      `json:"has_nsfw_content"`
	Categories     []string  `json:"categories"`
	Versions       []Version `json:"versions"`
}

type Version struct {
	Name          string        `json:"name"`
	FullName      string        `json:"full_name"`
	Description   string        `json:"description"`
	Icon          string        `json:"icon"`
	VersionNumber string        `json:"version_number"`
	Dependencies  []interface{} `json:"dependencies"`
	DownloadURL   string        `json:"download_url"`
	Downloads     int           `json:"downloads"`
	DateCreated   time.Time     `json:"date_created"`
	WebsiteURL    string        `json:"website_url"`
	IsActive      bool          `json:"is_active"`
	UUID4         string        `json:"uuid4"`
	FileSize      int           `json:"file_size"`
}

var thunderStoreLink = "https://northstar.thunderstore.io/api/v1/package/"

func GetPackages(ctx context.Context) ([]Package, error) {
	client := http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, thunderStoreLink, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed http call to get packages: %w", err)
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var packages []Package
	err = json.Unmarshal(body, &packages)
	if err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	return packages, nil
}

var ErrNoSuchPackage = fmt.Errorf("no such thunderstore package")

func GetPackageByName(ctx context.Context, name string) (Package, error) {
	packages, err := GetPackages(ctx)
	if err != nil {
		return Package{}, err
	}
	for _, pkg := range packages {
		if pkg.Name == name {
			return pkg, nil
		}
	}
	return Package{}, fmt.Errorf("%w: %s", ErrNoSuchPackage, name)
}

var ErrNoVersionsDetected = fmt.Errorf("no versions detected")

func GetLatestPackageVersion(pkg Package) (Version, error) {
	if len(pkg.Versions) > 0 {
		return pkg.Versions[0], nil
	}
	return Version{}, ErrNoVersionsDetected
}
