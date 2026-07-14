package handlers

import "strings"

type actorDefenseLiquidityEvidenceLine struct {
	Found            bool
	Parsed           bool
	Program          string
	PoolWallet       string
	InstructionTypes []string
}

// actorDefenseLiquidityEvidence inspects only explicit parsed fields for the
// hard-trigger evidence line. Opaque instruction account arrays are not
// semantically labelled as pools because that would turn position into proof.
func actorDefenseLiquidityEvidence(message, meta map[string]any) actorDefenseLiquidityEvidenceLine {
	instructionTypes, _ := creatorIntelInstructions(message, meta)
	result := actorDefenseLiquidityEvidenceLine{InstructionTypes: instructionTypes}
	for _, instruction := range actorDefenseInstructions(message, meta) {
		parsed := creatorIntelMap(instruction["parsed"])
		kind := strings.ToLower(strings.TrimSpace(creatorIntelCleanString(parsed["type"])))
		if !actorDefenseParsedLiquidityRemoval([]string{kind}) {
			continue
		}
		result.Found = true
		result.Parsed = true
		result.Program = firstNonEmptyString(
			creatorIntelCleanString(instruction["programId"]),
			creatorIntelCleanString(instruction["program"]),
		)
		info := creatorIntelMap(parsed["info"])
		result.PoolWallet = firstNonEmptyString(
			creatorIntelCleanString(info["pool"]),
			creatorIntelCleanString(info["poolAccount"]),
			creatorIntelCleanString(info["poolState"]),
			creatorIntelCleanString(info["amm"]),
			creatorIntelCleanString(info["ammId"]),
			creatorIntelCleanString(info["market"]),
		)
		return result
	}

	logs := strings.ToLower(strings.Join(creatorIntelStringSlice(meta["logMessages"]), "\n"))
	for _, marker := range []string{"remove_liquidity", "remove liquidity", "withdraw liquidity", "withdraw all token types"} {
		if strings.Contains(logs, marker) {
			result.Found = true
			return result
		}
	}
	return result
}

func (line actorDefenseLiquidityEvidenceLine) Complete() bool {
	return line.Found && line.Parsed && strings.TrimSpace(line.Program) != "" && strings.TrimSpace(line.PoolWallet) != ""
}
