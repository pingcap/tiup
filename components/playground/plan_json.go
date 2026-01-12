package main

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
)

type orderedStringIntMap map[string]int

func (m orderedStringIntMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodedKey, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(encodedKey)
		b.WriteByte(':')
		encodedValue, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		b.Write(encodedValue)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

type orderedStringConfigMap map[string]proc.Config

func (m orderedStringConfigMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodedKey, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(encodedKey)
		b.WriteByte(':')
		encodedValue, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		b.Write(encodedValue)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (p BootPlan) MarshalJSON() ([]byte, error) {
	type stableBootPlan struct {
		DataDir     string
		BootVersion string
		Host        string

		Shared proc.SharedOptions

		Monitor     bool
		GrafanaPort int

		RequiredServices orderedStringIntMap
		Downloads        []DownloadPlan
		Services         []ServicePlan

		DebugServiceConfigs orderedStringConfigMap
	}

	stable := stableBootPlan{
		DataDir:             p.DataDir,
		BootVersion:         p.BootVersion,
		Host:                p.Host,
		Shared:              p.Shared,
		Monitor:             p.Monitor,
		GrafanaPort:         p.GrafanaPort,
		RequiredServices:    orderedStringIntMap(p.RequiredServices),
		Downloads:           p.Downloads,
		Services:            p.Services,
		DebugServiceConfigs: orderedStringConfigMap(p.DebugServiceConfigs),
	}
	return json.Marshal(stable)
}
