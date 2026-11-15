package steamhook

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"regexp"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const UpdateCDN = "https://client-update.akamai.steamstatic.com"

var (
	buildIDPattern     = regexp.MustCompile(`^[0-9]+$`)
	packageFilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Release struct {
	ID              string
	Builds          []KnownBuild
	SkippedVariants []string
}

type cdnPackage struct {
	Name   string
	File   string
	SHA256 string
}

func FetchRelease(ctx context.Context, cdn string, cacheDir string) (Release, error) {
	manifest, err := fetchManifest(ctx, cdn)
	if err != nil {
		return Release{}, err
	}

	version := manifest.Path("osx", "version")
	if version == nil || !buildIDPattern.MatchString(version.Value) {
		return Release{}, failure.Errorf(failure.CodeBadSteamManifest, "Valve's manifest at %s has no numeric osx version", cdn)
	}
	client, err := readPackage(manifest.Path("osx", "bins_client_osx"), "bins_client_osx")
	if err != nil {
		return Release{}, err
	}
	variants, err := steamVariants(manifest.Path("osx", "steam_osx"))
	if err != nil {
		return Release{}, err
	}

	clientSHA256, hasClient, err := hashPackageFile(ctx, cdn, cacheDir, client, "steamclient.dylib")
	if err != nil {
		return Release{}, err
	}
	if !hasClient {
		return Release{}, failure.Errorf(failure.CodeBadSteamManifest, "%s has no steamclient.dylib", client.File)
	}

	release := Release{ID: version.Value}
	for _, variant := range variants {
		steamSHA256, hasSteam, err := hashPackageFile(ctx, cdn, cacheDir, variant, "steam_osx")
		if err != nil {
			return Release{}, err
		}
		if !hasSteam {
			release.SkippedVariants = append(release.SkippedVariants, variant.Name)
			continue
		}
		release.Builds = append(release.Builds, KnownBuild{ID: version.Value, Variant: variant.Name, Files: []KnownFile{
			{Path: "steamclient.dylib", SHA256: clientSHA256, Package: client.File},
			{Path: "steam_osx", SHA256: steamSHA256, Package: variant.File},
		}})
	}
	if len(release.Builds) == 0 {
		return Release{}, failure.Errorf(failure.CodeBadSteamManifest, "no steam_osx package of build %s holds a steam_osx", release.ID)
	}
	return release, nil
}

func fetchManifest(ctx context.Context, cdn string) (*Node, error) {
	url := cdn + "/steam_client_osx"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, failure.Errorf(failure.CodeDownloadFailed, "bad URL %s: %s", url, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, failure.Errorf(failure.CodeDownloadFailed, "downloading %s: %s", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, failure.Errorf(failure.CodeDownloadFailed, "downloading %s: HTTP %d", url, response.StatusCode)
	}

	text, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, failure.Errorf(failure.CodeDownloadFailed, "downloading %s: %s", url, err)
	}
	return ParseVDF(string(text))
}

func steamVariants(steam *Node) ([]cdnPackage, error) {
	if steam == nil {
		return nil, failure.Errorf(failure.CodeBadSteamManifest, "Valve's manifest has no steam_osx package")
	}

	var variants []cdnPackage
	for _, child := range steam.Children {
		if !child.IsObject {
			continue
		}
		variant, err := readPackage(child, child.Key)
		if err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	if steam.Child("file") == nil {
		return variants, nil
	}

	plain, err := readPackage(steam, "plain")
	if err != nil {
		return nil, err
	}
	return append(variants, plain), nil
}

func readPackage(node *Node, name string) (cdnPackage, error) {
	if node == nil {
		return cdnPackage{}, failure.Errorf(failure.CodeBadSteamManifest, "Valve's manifest has no %s package", name)
	}

	file := node.Child("file")
	if file == nil || !packageFilePattern.MatchString(file.Value) {
		return cdnPackage{}, failure.Errorf(failure.CodeBadSteamManifest, "the %s package in Valve's manifest has no plain file name", name)
	}
	sha2 := node.Child("sha2")
	if sha2 == nil || !sha256Pattern.MatchString(sha2.Value) {
		return cdnPackage{}, failure.Errorf(failure.CodeBadSteamManifest, "the %s package in Valve's manifest has no SHA-256 (sha2)", name)
	}
	return cdnPackage{Name: name, File: file.Value, SHA256: sha2.Value}, nil
}

func hashPackageFile(ctx context.Context, cdn string, cacheDir string, pkg cdnPackage, name string) (string, bool, error) {
	path, err := wineruntime.Download(ctx, cacheDir, wineruntime.Archive{Name: pkg.File, URL: cdn + "/" + pkg.File, SHA256: pkg.SHA256})
	if err != nil {
		return "", false, err
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", false, failure.Errorf(failure.CodeExtractFailed, "could not open %s: %v", pkg.File, err)
	}
	defer archive.Close()

	data, err := fs.ReadFile(archive, name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, failure.Errorf(failure.CodeExtractFailed, "could not read %s from %s: %v", name, pkg.File, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), true, nil
}
