package services

import "strings"

func isKnownRaydiumProgram(program string) bool {
	program = strings.TrimSpace(program)
	if program == "" {
		return false
	}
	return program == defaultRaydiumProgramID ||
		program == legacyRaydiumProgramID ||
		program == legacyRaydiumSourceID ||
		strings.Contains(strings.ToLower(program), "raydium")
}
