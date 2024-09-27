package thunderstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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
		Timeout: time.Second * 10,
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
var ErrPackageNameCollision = fmt.Errorf("multiple packages found with the same name")

func GetPackageByName(ctx context.Context, name string) (Package, error) {
	packages, err := GetPackages(ctx)
	if err != nil {
		return Package{}, err
	}

	// if name contains /, then split into name and owner
	// then check if owner matches and name matches
	var owner string
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		owner = parts[0]
		name = parts[1]
	}
	// array of string of different owners
	owners := []string{}
	//pointer to a package
	pkgG := &Package{}

	for _, pkg := range packages {
		if owner != "" {
			// If owner is specified, check both owner and name
			if pkg.Owner == owner && pkg.Name == name {
				return pkg, nil
			}
		} else {
			// If no owner specified, just check the name
			if pkg.Name == name {
				owners = append(owners, pkg.Owner)
				pkgG = &pkg
			}
		}
	}

	if len(owners) == 0 {
		return Package{}, fmt.Errorf("%w: %s", ErrNoSuchPackage, name)
	} else if len(owners) > 1 {
		errString := fmt.Sprintf("multiple packages found with name %s. Please prepend the owner name, like this: Owner/PackageName. Avaliable options are:", name)
		for _, owner := range owners {
			errString += "\n" + owner + "/" + name
		}
		return Package{}, fmt.Errorf("%w: %s", ErrPackageNameCollision, errString)
	} else {
		return *pkgG, nil
	}
}

var ErrNoVersionsDetected = fmt.Errorf("no versions detected")

func GetLatestPackageVersion(pkg Package) (Version, error) {
	if len(pkg.Versions) > 0 {
		return pkg.Versions[0], nil
	}
	return Version{}, ErrNoVersionsDetected
}
