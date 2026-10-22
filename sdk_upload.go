package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"strings"
	"time"

	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
)

// sdkManifest mirrors the subset of the SDK package manifest schema
// (HA-Contract sdk-package.md) WinnerProxy needs to drive the upload.
type sdkManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// loadSDKManifest reads the embedded manifest.json.
func loadSDKManifest() (sdkManifest, error) {
	var m sdkManifest
	data, err := fs.ReadFile(sdkSource, "sdk/manifest.json")
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse embedded manifest.json: %w", err)
	}
	return m, nil
}

// buildSDKArchive packs the embedded sdk/ source tree into an in-memory
// tar.gz archive with manifest.json at its root, matching the layout
// validated by HRPAuth (see sdk-package.md).
func buildSDKArchive() ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// embed.FS paths start with "sdk/"; strip that prefix so the archive
	// carries manifest.json at the root and src/... with relative paths.
	if err := fs.WalkDir(sdkSource, "sdk", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(sdkSource, p)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    strings.TrimPrefix(p, "sdk/"),
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	}); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// uploadSDKPackage uploads the compile-time SDK package to HRPAuth
// asynchronously (POST /services/sdk-packages, field "package"). If a
// package with the same name already exists (409), the stale package is
// deleted and the upload retried once, so every restart converges the
// remote package to the embedded sources. Failures are only logged and
// never block or stop the proxy.
func uploadSDKPackage(cli *hrpauth.Client) {
	go func() {
		log.Printf("SDK: starting sdk package aggregation process")
		manifest, err := loadSDKManifest()
		if err != nil {
			log.Printf("SDK: load embedded manifest failed: %v", err)
			return
		}
		log.Printf("SDK: manifest loaded (name=%s, version=%s)", manifest.Name, manifest.Version)

		archive, err := buildSDKArchive()
		if err != nil {
			log.Printf("SDK: archive build failed: %v", err)
			return
		}
		log.Printf("SDK: archive built successfully (%d bytes)", len(archive))

		filename := manifest.Name + "-sdk.tar.gz"
		if err := cli.UploadSDKPackage(archive, filename); err != nil {
			if !errors.Is(err, hrpauth.ErrSDKPackageConflict) {
				log.Printf("SDK: upload to HRPAuth failed: %v", err)
				return
			}
			log.Printf("SDK: package %q already exists, performing stale cleanup...", manifest.Name)
			if delErr := cli.DeleteSDKPackage(manifest.Name); delErr != nil {
				log.Printf("SDK: cleanup failed: %v", delErr)
				return
			}
			log.Printf("SDK: stale package deleted, retrying upload...")
			if err := cli.UploadSDKPackage(archive, filename); err != nil {
				log.Printf("SDK: re-upload failed: %v", err)
				return
			}
		}
		log.Printf("SDK: package %q (v%s) successfully aggregated into HRPAuth", manifest.Name, manifest.Version)
	}()
}
