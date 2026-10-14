package solana

import (
	"encoding/binary"
	"testing"
)

func TestUpgradeableProgramState(t *testing.T) {
	program := make([]byte, 36)
	binary.LittleEndian.PutUint32(program, ProgramState)
	program[4] = 7
	id, err := DecodeProgram(program)
	if err != nil || id[0] != 7 {
		t.Fatalf("program address decode: %v", err)
	}
	if _, err := DecodeProgram(program[:35]); err == nil {
		t.Fatal("truncated program accepted")
	}
}

func TestProgramDataAuthorityStates(t *testing.T) {
	data := make([]byte, 47)
	binary.LittleEndian.PutUint32(data, ProgramDataState)
	binary.LittleEndian.PutUint64(data[4:], 123)
	data[45], data[46] = 1, 2
	state, err := DecodeProgramData(data)
	if err != nil || state.Authority != nil || state.Slot != 123 || len(state.Bytecode) != 2 {
		t.Fatalf("revoked authority decode: %+v %v", state, err)
	}
	data[12], data[13] = 1, 9
	state, err = DecodeProgramData(data)
	if err != nil || state.Authority == nil || state.Authority[0] != 9 {
		t.Fatalf("present authority decode: %+v %v", state, err)
	}
	data[12] = 2
	if _, err := DecodeProgramData(data); err == nil {
		t.Fatal("malformed option accepted")
	}
	if _, err := DecodeProgramData(data[:44]); err == nil {
		t.Fatal("truncated programdata accepted")
	}
}
