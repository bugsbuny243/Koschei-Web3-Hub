package solana

import (
	"context"
	"errors"
	radar "koschei/api/internal/radaradapter"
)

// ProgramObservation contains two separately fetched finalized account values.
// The slot is a lower bound; no atomicity or historical state is asserted.
type ProgramObservation struct {
	Slot uint64
	Program Account
	ProgramData Account
	State ProgramData
	ProgramResponse radar.Response
	DataResponse radar.Response
}

func ObserveProgram(ctx context.Context, t radar.Transport, network, programAddress, dataAddress string) (ProgramObservation, error) {
	var out ProgramObservation
	slot, err := ReadFinalizedSlot(ctx, t, network)
	if err != nil { return out, err }
	out.Slot = slot
	out.ProgramResponse, out.Program, err = ReadAccountAtSlot(ctx, t, network, programAddress, slot)
	if err != nil { return ProgramObservation{}, err }
	linked, err := DecodeUpgradeableProgramAccount(out.Program)
	if err != nil { return ProgramObservation{}, err }
	decoded, err := DecodeSolanaPubkey(dataAddress)
	if err != nil || linked != decoded { return ProgramObservation{}, errors.New("programdata address does not match program link") }
	out.DataResponse, out.ProgramData, err = ReadAccountAtSlot(ctx, t, network, dataAddress, slot)
	if err != nil { return ProgramObservation{}, err }
	out.State, err = DecodeUpgradeableProgramDataAccount(out.ProgramData)
	if err != nil { return ProgramObservation{}, err }
	if out.State.Slot > slot { return ProgramObservation{}, errors.New("upgrade slot newer than finalized anchor") }
	return out, nil
}
