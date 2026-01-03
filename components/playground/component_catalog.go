package main

import (
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

type componentVersionBinder func(p *Playground, bootVersion string) string

var componentVersionBinders = map[proc.RepoComponentID]componentVersionBinder{
	proc.ComponentPrometheus: func(_ *Playground, bootVersion string) string {
		return strings.TrimSuffix(bootVersion, "-"+utils.NextgenVersionAlias)
	},
	proc.ComponentGrafana: func(_ *Playground, bootVersion string) string {
		return strings.TrimSuffix(bootVersion, "-"+utils.NextgenVersionAlias)
	},
	proc.ComponentTiKVCDC: func(p *Playground, bootVersion string) string {
		if p == nil || p.bootOptions == nil {
			return bootVersion
		}
		if v := p.bootOptions.Service(proc.ServiceTiKVCDC).Version; v != "" {
			return v
		}
		return bootVersion
	},
	proc.ComponentTiProxy: func(p *Playground, bootVersion string) string {
		if p == nil || p.bootOptions == nil {
			return bootVersion
		}
		if v := p.bootOptions.Service(proc.ServiceTiProxy).Version; v != "" {
			return v
		}
		return bootVersion
	},
}
