package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/sync/errgroup"
)

type bootExecutor struct {
	pg  *Playground
	src ComponentSource
}

func newBootExecutor(pg *Playground, src ComponentSource) *bootExecutor {
	return &bootExecutor{pg: pg, src: src}
}

type preRunHandler func(ctx context.Context, plan BootPlan, services []ServicePlan) error

var servicePreRunHandlers = map[proc.ServiceID]preRunHandler{
	proc.ServiceTiProxy: func(_ context.Context, plan BootPlan, _ []ServicePlan) error {
		return proc.GenTiProxySessionCerts(plan.DataDir)
	},
}

// maxParallelComponentDownloads caps the number of concurrent component
// downloads during boot.
const maxParallelComponentDownloads = 4

// Download installs components required by the boot plan.
//
// This implementation is intentionally more complex than a simple loop calling
// src.EnsureInstalled().
//
// Background / constraints:
//   - The repository layer reports progress via repository.DownloadProgress which
//     only models a *single* active download (Start/SetCurrent/Finish). Our UI
//     adapter (repoDownloadProgress) also keeps mutable "current download" state
//     (e.g. `task`, throttling fields).
//   - To show multiple downloads concurrently in the TUI we need *one progress
//     instance per download*, otherwise different downloads would race on the
//     shared state and the UI would mix/flicker.
//   - envComponentSource.EnsureInstalled delegates to env.V1Repository().UpdateComponents,
//     which is primarily designed for sequential installation and shares a single
//     mirror/progress instance internally. Simply calling it concurrently would
//     not yield the desired UI and would also introduce unnecessary contention
//     around manifest updates.
//
// Approach:
//   - Prefetch version items (URL + SHA256) serially, so manifest fetch/update
//     happens deterministically and without concurrent writes to local manifests.
//   - Download + verify + untar in parallel with an errgroup limit, using a fresh
//     mirror/progress instance per goroutine.
//
// Note: this logic intentionally lives in playground-ng to avoid invasive
// changes to shared pkg/repository APIs.
func (e *bootExecutor) Download(ctx context.Context, plan BootPlan) error {
	if e == nil || e.src == nil {
		return errors.New("component source not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	downloads := normalizeDownloadPlans(plan.Downloads)
	if len(downloads) == 0 {
		return nil
	}

	var (
		progressFactory func() repository.DownloadProgress
	)
	if e.pg != nil {
		progressFactory = e.pg.downloadProgressFactory()
	}
	if progressFactory == nil {
		progressFactory = func() repository.DownloadProgress { return repository.DisableProgress{} }
	}

	if src, ok := e.src.(*envComponentSource); ok && src != nil && src.env != nil {
		repo := src.env.V1Repository()
		if repo == nil {
			return errors.New("repository not initialized")
		}
		mirror := repo.Mirror()
		if mirror == nil || strings.TrimSpace(mirror.Source()) == "" {
			return errors.New("repository mirror not initialized")
		}
		profile := src.env.Profile()
		if profile == nil {
			return errors.New("profile not initialized")
		}

		disableDecompress := false
		if r, ok := repo.(*repository.V1Repository); ok {
			disableDecompress = r.DisableDecompress
		}

		keepSource := false
		if v := os.Getenv(localdata.EnvNameKeepSourceTarget); v == "enable" || v == "true" {
			keepSource = true
		}

		prepared := make([]preparedDownload, 0, len(downloads))
		for _, d := range downloads {
			item, err := repo.ComponentVersion(d.ComponentID, d.ResolvedVersion, false)
			if err != nil {
				return err
			}
			if item == nil || strings.TrimSpace(item.URL) == "" {
				return errors.Errorf("missing download URL for component %s:%s", d.ComponentID, d.ResolvedVersion)
			}
			if item.Hashes == nil || strings.TrimSpace(item.Hashes[v1manifest.SHA256]) == "" {
				return errors.Errorf("missing sha256 hash for component %s:%s", d.ComponentID, d.ResolvedVersion)
			}
			prepared = append(prepared, preparedDownload{
				component: d.ComponentID,
				version:   d.ResolvedVersion,
				item:      item,
			})
		}

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(maxParallelComponentDownloads)
		for _, d := range prepared {
			d := d
			g.Go(func() error {
				return downloadAndInstallComponent(gctx, mirror.Source(), profile, d, downloadInstallOptions{
					disableDecompress: disableDecompress,
					keepSource:        keepSource,
					progress:          progressFactory(),
				})
			})
		}
		return g.Wait()
	}

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(maxParallelComponentDownloads)
	for _, d := range downloads {
		d := d
		g.Go(func() error {
			return e.src.EnsureInstalled(d.ComponentID, d.ResolvedVersion)
		})
	}
	return g.Wait()
}

func (e *bootExecutor) PreRun(ctx context.Context, plan BootPlan) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := preflightBootPlan(ctx, plan); err != nil {
		return err
	}

	enabledServices := make(map[proc.ServiceID][]ServicePlan, len(plan.Services))
	for _, svc := range plan.Services {
		id := proc.ServiceID(strings.TrimSpace(svc.ServiceID))
		if id == "" {
			continue
		}
		enabledServices[id] = append(enabledServices[id], svc)
	}

	handlerKeys := make([]proc.ServiceID, 0, len(servicePreRunHandlers))
	for serviceID := range servicePreRunHandlers {
		if serviceID != "" {
			handlerKeys = append(handlerKeys, serviceID)
		}
	}
	slices.SortFunc(handlerKeys, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range handlerKeys {
		servicePlans := enabledServices[serviceID]
		if len(servicePlans) == 0 {
			continue
		}
		h := servicePreRunHandlers[serviceID]
		if h == nil {
			continue
		}
		if err := h(ctx, plan, servicePlans); err != nil {
			return err
		}
	}

	return nil
}

func (e *bootExecutor) AddProcs(ctx context.Context, plan BootPlan) error {
	if e == nil || e.pg == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	type binKey struct {
		component string
		version   string
	}
	binCache := make(map[binKey]string)

	for _, svc := range plan.Services {
		if strings.TrimSpace(svc.ServiceID) == "" {
			continue
		}

		serviceID := proc.ServiceID(strings.TrimSpace(svc.ServiceID))

		binPath := svc.BinPath
		if binPath == "" {
			if e.src == nil {
				return errors.New("component source not initialized")
			}
			if svc.ComponentID == "" || svc.ResolvedVersion == "" {
				return errors.Errorf("missing component identity for service %s(%s)", svc.ServiceID, svc.Name)
			}
			key := binKey{component: svc.ComponentID, version: svc.ResolvedVersion}
			if cached := binCache[key]; cached != "" {
				binPath = cached
			} else {
				resolved, err := e.src.BinaryPath(svc.ComponentID, svc.ResolvedVersion)
				if err != nil {
					return err
				}
				binPath = resolved
				binCache[key] = resolved
			}

			baseBinPath := binPath
			required := requiredBinaryPathForService(serviceID, baseBinPath)
			if required == "" || !binaryExists(required) {
				return errors.Errorf("binary for service %s not found: %s (base: %s)", serviceID, required, baseBinPath)
			}
			binPath = required
		}

		v := utils.Version(strings.TrimSpace(svc.ResolvedVersion))
		if v.IsEmpty() {
			return errors.Errorf("missing planned version for service %s(%s)", svc.ServiceID, svc.Name)
		}

		if _, err := e.pg.requestAddPlannedProc(ctx, svc, binPath, v, plan.Shared, plan.DataDir); err != nil {
			return err
		}
	}

	return nil
}

func preflightBootPlan(ctx context.Context, plan BootPlan) error {
	if ctx == nil {
		ctx = context.Background()
	}
	switch plan.Shared.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
	default:
		return nil
	}

	bucket := strings.TrimSpace(plan.Shared.CSE.Bucket)
	if bucket == "" {
		return fmt.Errorf("missing s3 bucket")
	}

	rawEndpoint, isSecure, _, err := parseS3Endpoint(plan.Shared.CSE.S3Endpoint)
	if err != nil {
		return err
	}

	s3Client, err := minio.New(rawEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(plan.Shared.CSE.AccessKey, plan.Shared.CSE.SecretKey, ""),
		Secure: isSecure,
	})
	if err != nil {
		return errors.Annotate(err, "can not connect to S3 endpoint")
	}

	ctxCheck, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bucketExists, err := s3Client.BucketExists(ctxCheck, bucket)
	if err != nil {
		return errors.Annotate(err, "can not connect to S3 endpoint")
	}
	if bucketExists {
		return nil
	}
	if err := s3Client.MakeBucket(ctxCheck, bucket, minio.MakeBucketOptions{}); err != nil {
		return errors.Errorf("cannot create s3 bucket: Bucket %s doesn't exist and fail to create automatically (your bucket name may be invalid?)", bucket)
	}
	return nil
}

type preparedDownload struct {
	component string
	version   string
	item      *v1manifest.VersionItem
}

type downloadInstallOptions struct {
	disableDecompress bool
	keepSource        bool
	progress          repository.DownloadProgress
}

// downloadAndInstallComponent is a minimal, playground-ng-local
// reimplementation of the "download + verify + (optional) untar" part of
// repository.UpdateComponents.
//
// It exists because parallel boot downloads need:
// - per-download progress reporters (DownloadProgress is single-task), and
// - per-download mirror instances (each mirror has its own tempdir/progress),
// without expanding shared pkg/repository APIs just for playground-ng.
//
// It relies on ComponentVersion() having been called earlier so the local
// component manifest exists (for later BinaryPath resolution via Entry).
func downloadAndInstallComponent(ctx context.Context, mirrorSource string, profile *localdata.Profile, d preparedDownload, opt downloadInstallOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(mirrorSource) == "" {
		return errors.New("mirror source is empty")
	}
	if profile == nil {
		return errors.New("profile is nil")
	}
	if strings.TrimSpace(d.component) == "" || strings.TrimSpace(d.version) == "" || d.item == nil {
		return errors.New("download plan is invalid")
	}

	installDir := profile.Path(localdata.ComponentParentDir, d.component, d.version)
	target := filepath.Join(installDir, d.item.URL)

	mirror := repository.NewMirror(mirrorSource, repository.MirrorOptions{
		Context:  ctx,
		Progress: opt.progress,
	})
	if err := mirror.Open(); err != nil {
		return err
	}
	defer func() { _ = mirror.Close() }()

	if err := mirror.Download(d.item.URL, installDir); err != nil {
		_ = os.RemoveAll(installDir)
		return err
	}

	expected := strings.TrimSpace(d.item.Hashes[v1manifest.SHA256])
	if expected == "" {
		_ = os.RemoveAll(installDir)
		return errors.Errorf("missing sha256 hash for %s", target)
	}
	if err := verifySHA256(target, expected); err != nil {
		_ = os.RemoveAll(installDir)
		return errors.Errorf("validation failed for %s: %s", target, err)
	}

	if !opt.disableDecompress {
		reader, err := os.Open(target)
		if err != nil {
			_ = os.RemoveAll(installDir)
			return err
		}
		err = utils.Untar(reader, installDir)
		_ = reader.Close()
		if err != nil {
			_ = os.RemoveAll(installDir)
			return err
		}
	}

	if !opt.disableDecompress && !opt.keepSource {
		_ = os.Remove(target)
	}

	return nil
}

func verifySHA256(path string, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return errors.New("expected sha256 is empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch (expect %s, got %s)", expected, actual)
	}
	return nil
}

// normalizeDownloadPlans trims/filters and de-duplicates download plans.
//
// Boot planning can produce multiple service instances that depend on the same
// component@version. Downloading the same tarball more than once would waste
// bandwidth and also makes the progress UI misleading.
func normalizeDownloadPlans(plans []DownloadPlan) []DownloadPlan {
	if len(plans) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(plans))
	normalized := make([]DownloadPlan, 0, len(plans))
	for _, d := range plans {
		component := strings.TrimSpace(d.ComponentID)
		resolved := strings.TrimSpace(d.ResolvedVersion)
		if component == "" || resolved == "" {
			continue
		}
		key := component + "@" + resolved
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, DownloadPlan{
			ComponentID:     component,
			ResolvedVersion: resolved,
		})
	}
	return normalized
}
