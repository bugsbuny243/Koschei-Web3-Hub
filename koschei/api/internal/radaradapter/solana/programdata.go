// Package solana decodes upgradeable BPF loader state without claiming live RPC availability.
package solana

import (
	"encoding/binary"
	"errors"
)

const (
	ProgramState = uint32(2)
	ProgramDataState = uint32(3)
)

// ProgramData is the upgradeable loader's on-chain ProgramData account header.
// Authority nil means the authority was explicitly revoked, not unreadable.
type ProgramData struct {
	Slot      uint64
	Authority *[32]byte
	Bytecode  []byte
}

// DecodeProgram requires the exact 36-byte program account state.
func DecodeProgram(data []byte) ([32]byte, error) {
	var address [32]byte
	if len(data) != 36 || binary.LittleEndian.Uint32(data[:4]) != ProgramState {
		return address, errors.New("invalid upgradeable program account")
	}
	copy(address[:], data[4:])
	return address, nil
}

// DecodeProgramData parses the documented 45-byte metadata prefix.
func DecodeProgramData(data []byte) (ProgramData, error) {
	var out ProgramData
	if len(data) < 45 || binary.LittleEndian.Uint32(data[:4]) != ProgramDataState {
		return out, errors.New("invalid programdata account")
	}
	out.Slot = binary.LittleEndian.Uint64(data[4:12])
	switch data[12] {
	case 0:
		// The 32 reserved bytes are not an authority when Option::None.
	case 1:
		var key [32]byte
		copy(key[:], data[13:45])
		out.Authority = &key
	default:
		return ProgramData{}, errors.New("invalid upgrade authority option")
	}
	out.Bytecode = append([]byte(nil), data[45:]...)
	if len(out.Bytecode) == 0 {
		return ProgramData{}, errors.New("program bytecode unavailable")
	}
	return out, nil
}
