package handlers

func normalizePlanTier(planTier string) string {
	return normalizePackageID(planTier)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
