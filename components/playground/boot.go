package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/utils"
)

func (p *Playground) setRequiredServices(options *BootOptions) {
	if p == nil || options == nil {
		return
	}

	required := make(map[proc.ServiceID]int)

	for _, def := range serviceCatalog {
		if def.CriticalWhen == nil || !def.CriticalWhen(options) {
			continue
		}
		cfg := options.Service(def.ServiceID)
		if cfg == nil || cfg.Num <= 0 {
			continue
		}
		required[def.ServiceID] = 1
	}

	p.requiredServices = required
	p.criticalRunning = make(map[proc.ServiceID]int)
}

func (p *Playground) isRequiredService(serviceID proc.ServiceID) bool {
	if p == nil || serviceID == "" {
		return false
	}
	min := p.requiredServices[serviceID]
	return min > 0
}

func normalizeBootErr(ctx context.Context, err error) error {
	if err == nil || ctx == nil {
		return err
	}

	if stdErrors.Is(err, context.Canceled) || errors.Cause(err) == context.Canceled {
		cause := context.Cause(ctx)
		if cause != nil && cause != context.Canceled {
			return cause
		}
	}
	return err
}

func (p *Playground) cancelBootWithCause(cause error) {
	if p == nil {
		return
	}
	cancel := p.bootCancel
	if cancel != nil {
		cancel(cause)
	}
}

func (p *Playground) normalizeBootOptionPaths(options *BootOptions) error {
	if options == nil {
		return nil
	}

	for _, serviceID := range options.SortedServiceIDs() {
		cfg := options.Service(serviceID)
		if cfg == nil {
			continue
		}
		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	return nil
}

func (p *Playground) validateBootOptions(ctx context.Context, options *BootOptions) error {
	if p == nil || options == nil {
		return nil
	}

	cfgPD := options.Service(proc.ServicePD)
	cfgDMMaster := options.Service(proc.ServiceDMMaster)
	cfgTiKVWorker := options.Service(proc.ServiceTiKVWorker)
	cfgTiDBSystem := options.Service(proc.ServiceTiDBSystem)
	cfgTiFlash := options.Service(proc.ServiceTiFlash)
	cfgTiFlashWrite := options.Service(proc.ServiceTiFlashWrite)
	cfgTiFlashCompute := options.Service(proc.ServiceTiFlashCompute)

	// All other components depend on PD, except DM. Ensure PD count > 0 for the
	// common modes.
	if options.ShOpt.PDMode != "ms" && cfgPD != nil && cfgPD.Num < 1 && cfgDMMaster != nil && cfgDMMaster.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (pd=%v)", cfgPD.Num)
	}

	if cfgTiKVWorker != nil && cfgTiKVWorker.Num > 1 {
		return fmt.Errorf("TiKV worker only supports at most 1 instance")
	}
	if cfgTiDBSystem != nil && cfgTiDBSystem.Num > 1 {
		return fmt.Errorf("TiDB system only supports at most 1 instance")
	}

	if utils.Version(options.Version).IsValid() {
		hasTiFlash := false
		if cfgTiFlash != nil && cfgTiFlash.Num != 0 {
			hasTiFlash = true
		}
		if cfgTiFlashWrite != nil && cfgTiFlashWrite.Num != 0 {
			hasTiFlash = true
		}
		if cfgTiFlashCompute != nil && cfgTiFlashCompute.Num != 0 {
			hasTiFlash = true
		}

		if hasTiFlash && !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			colorstr.Fprintf(p.termWriter(), "[yellow][bold]Warning:[reset] TiFlash requires TiDB >= v7.1.0 (or nightly); disabling TiFlash for version %s\n", options.Version)
			if cfgTiFlash != nil {
				cfgTiFlash.Num = 0
			}
			if cfgTiFlashWrite != nil {
				cfgTiFlashWrite.Num = 0
			}
			if cfgTiFlashCompute != nil {
				cfgTiFlashCompute.Num = 0
			}
		}
	}

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "https://") && !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "http://") {
			return fmt.Errorf("require S3 endpoint to start with http:// or https://")
		}

		isSecure := strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "https://")
		rawEndpoint := strings.TrimPrefix(options.ShOpt.CSE.S3Endpoint, "https://")
		rawEndpoint = strings.TrimPrefix(rawEndpoint, "http://")

		// Currently we always assign region=local. Other regions are not supported.
		if strings.Contains(rawEndpoint, "amazonaws.com") {
			return fmt.Errorf("Currently TiUP playground only supports local S3 (like minio). S3 on AWS Regions are not supported. Contributions are welcome!")
		}

		// Preflight check whether specified object storage is available.
		s3Client, err := minio.New(rawEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(options.ShOpt.CSE.AccessKey, options.ShOpt.CSE.SecretKey, ""),
			Secure: isSecure,
		})
		if err != nil {
			return errors.Annotate(err, "can not connect to S3 endpoint")
		}

		ctxCheck, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		bucketExists, err := s3Client.BucketExists(ctxCheck, options.ShOpt.CSE.Bucket)
		if err != nil {
			return errors.Annotate(err, "can not connect to S3 endpoint")
		}
		if !bucketExists {
			// Try to create bucket.
			if err := s3Client.MakeBucket(ctxCheck, options.ShOpt.CSE.Bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("cannot create s3 bucket: Bucket %s doesn't exist and fail to create automatically (your bucket name may be invalid?)", options.ShOpt.CSE.Bucket)
			}
		}
	}

	return nil
}

type plannedProc struct {
	serviceID proc.ServiceID
	cfg       proc.Config
}

func planProcs(options *BootOptions) ([]plannedProc, error) {
	if options == nil {
		return nil, nil
	}

	if options.ShOpt.PDMode == "ms" && !tidbver.PDSupportMicroservices(options.Version) {
		return nil, fmt.Errorf("PD cluster doesn't support microservices mode in version %s", options.Version)
	}

	if options.ShOpt.Mode == proc.ModeCSE || options.ShOpt.Mode == proc.ModeNextGen || options.ShOpt.Mode == proc.ModeDisAgg {
		if utils.Version(options.Version).IsValid() && !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			// For simplicity, currently we only implemented disagg mode when TiFlash can run without config.
			return nil, fmt.Errorf("TiUP playground only supports CSE/Disagg mode for TiDB cluster >= v7.1.0 (or nightly)")
		}
	}

	cfgByService := make(map[proc.ServiceID]proc.Config)
	var serviceIDs []proc.ServiceID

	for _, def := range serviceCatalog {
		if def.PlanWhen == nil || !def.PlanWhen(options) {
			continue
		}

		cfg := proc.Config{}
		if def.PlanConfig != nil {
			cfg = def.PlanConfig(options)
		} else {
			cfg, _ = options.ServiceConfig(def.ServiceID)
		}

		cfgByService[def.ServiceID] = cfg
		serviceIDs = append(serviceIDs, def.ServiceID)
	}

	ordered, err := topoSortServiceIDs(serviceIDs)
	if err != nil {
		return nil, err
	}

	plans := make([]plannedProc, 0, len(ordered))
	for _, serviceID := range ordered {
		plans = append(plans, plannedProc{serviceID: serviceID, cfg: cfgByService[serviceID]})
	}
	return plans, nil
}

func (p *Playground) addPlannedProcs(plans []plannedProc) error {
	if p == nil {
		return nil
	}
	for _, plan := range plans {
		for i := 0; i < plan.cfg.Num; i++ {
			_, err := p.addProc(plan.serviceID, plan.cfg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
