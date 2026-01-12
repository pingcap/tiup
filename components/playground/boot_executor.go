package main

import (
	"context"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

type bootExecutor struct {
	pg  *Playground
	src ComponentSource
}

func newBootExecutor(pg *Playground, src ComponentSource) *bootExecutor {
	return &bootExecutor{pg: pg, src: src}
}

func (e *bootExecutor) Download(plan BootPlan) error {
	if e == nil || e.src == nil {
		return errors.New("component source not initialized")
	}
	for _, d := range plan.Downloads {
		if d.ComponentID == "" || d.ResolvedVersion == "" {
			continue
		}
		if err := e.src.EnsureInstalled(d.ComponentID, d.ResolvedVersion); err != nil {
			return err
		}
	}
	return nil
}

func (e *bootExecutor) PreRun(ctx context.Context, plan BootPlan) error {
	if e == nil {
		return nil
	}

	if err := preflightBootPlan(ctx, plan); err != nil {
		return err
	}

	needsTiProxyCert := false
	for _, svc := range plan.Services {
		if svc.ServiceID == proc.ServiceTiProxy.String() {
			needsTiProxyCert = true
			break
		}
	}
	if needsTiProxyCert {
		if err := proc.GenTiProxySessionCerts(plan.DataDir); err != nil {
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
		}

		if serviceID == proc.ServiceTiKVWorker {
			binPath = proc.ResolveTiKVWorkerBinPath(binPath)
		}

		v := utils.Version(strings.TrimSpace(svc.ResolvedVersion))
		if v.IsEmpty() {
			// For user binaries, we may only know the configured constraint.
			// It is still important for feature gates during execution.
			v = utils.Version(strings.TrimSpace(svc.DebugConstraint))
		}

		if _, err := e.pg.requestAddPlannedProc(ctx, svc, binPath, v, plan.Shared, plan.DataDir); err != nil {
			return err
		}
	}

	return nil
}

func preflightBootPlan(ctx context.Context, plan BootPlan) error {
	switch plan.Shared.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
	default:
		return nil
	}

	endpoint := plan.Shared.CSE.S3Endpoint
	isSecure := strings.HasPrefix(endpoint, "https://")
	rawEndpoint := strings.TrimPrefix(endpoint, "https://")
	rawEndpoint = strings.TrimPrefix(rawEndpoint, "http://")

	s3Client, err := minio.New(rawEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(plan.Shared.CSE.AccessKey, plan.Shared.CSE.SecretKey, ""),
		Secure: isSecure,
	})
	if err != nil {
		return errors.Annotate(err, "can not connect to S3 endpoint")
	}

	ctxCheck, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bucketExists, err := s3Client.BucketExists(ctxCheck, plan.Shared.CSE.Bucket)
	if err != nil {
		return errors.Annotate(err, "can not connect to S3 endpoint")
	}
	if bucketExists {
		return nil
	}
	if err := s3Client.MakeBucket(ctxCheck, plan.Shared.CSE.Bucket, minio.MakeBucketOptions{}); err != nil {
		return errors.Errorf("cannot create s3 bucket: Bucket %s doesn't exist and fail to create automatically (your bucket name may be invalid?)", plan.Shared.CSE.Bucket)
	}
	return nil
}
