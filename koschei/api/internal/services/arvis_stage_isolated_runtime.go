package services

import "koschei/api/internal/runtimecfg"

func arvisGraphRuntimeEnabled() bool {
	return runtimecfg.ModuleEnabled(ModuleIntelligenceGraph)
}
