package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
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
