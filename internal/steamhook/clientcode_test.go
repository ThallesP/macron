package steamhook

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

const (
	fakeTextAddress      = 0x4000
	fakeStringsAddress   = 0x8000
	fakeSelectorsAddress = 0x9000
	fakeStubsAddress     = 0xa000
	fakeSelrefsAddress   = 0xc000
	fakeFlagOffset       = 0x810
	fakeUUID             = "00112233445566778899aabbccddeeff"
)

type fakeClient struct {
	enabledSites   int
	hasStop        bool
	hasProfileName bool
}

var wholeFakeClient = fakeClient{enabledSites: 1, hasStop: true, hasProfileName: true}

func adrp(register uint32, from uint64, target uint64) uint32 {
	pages := uint32((int64(target&^0xfff) - int64(from&^0xfff)) >> 12)
	return 0x90000000 | pages&3<<29 | pages>>2&0x7ffff<<5 | register
}

func addImmediate(register uint32, immediate uint64) uint32 {
	return 0x91000000 | uint32(immediate)<<10 | register<<5 | register
}

func branchWithLink(from uint64, to uint64) uint32 {
	return 0x94000000 | uint32((int64(to)-int64(from))/4)&0x3ffffff
}

func testBit0OfW0(from uint64, to uint64) uint32 {
	return 0x37000000 | uint32((int64(to)-int64(from))/4)&0x3fff<<5
}

const (
	returnInstruction = 0xd65f03c0
	noOperation       = 0xd503201f
)

// A Mach-O with the shapes FindClientCode looks for: the function that reads
// the enabled flag, the constructor that sets it from "linux", the terminate
// helper behind an Objective-C stub, and the function that gives up when it
// fails.
func (f fakeClient) build() []byte {
	strings := []byte("other\x00notlinux\x00linux\x00")
	profileNameOffset := uint64(len(strings))
	if f.hasProfileName {
		strings = append(strings, enabledProfileName+"\x00"...)
	}
	linuxAddress := uint64(fakeStringsAddress + bytes.Index(strings, []byte("\x00linux\x00")) + 1)
	selectors := []byte("activate\x00terminate\x00")
	terminateName := uint64(fakeSelectorsAddress + bytes.Index(selectors, []byte("terminate")))

	var text []uint32
	var starts []uint64
	here := func() uint64 { return fakeTextAddress + uint64(len(text))*4 }
	function := func() { starts = append(starts, here()) }

	function()
	text = append(text, adrp(0, here(), fakeStringsAddress+profileNameOffset))
	text = append(text, addImmediate(0, (fakeStringsAddress+profileNameOffset)&0xfff))
	text = append(text, 0x39400000|4<<10|19<<5|8, 0x39400000|fakeFlagOffset<<10|19<<5|8, returnInstruction)

	for range f.enabledSites {
		function()
		text = append(text, adrp(1, here(), linuxAddress))
		text = append(text, addImmediate(1, linuxAddress&0xfff), noOperation)
		text = append(text, branchWithLink(here(), fakeTextAddress), compareW0WithZero, 0x1a9f17e8, storeByte(8, fakeFlagOffset)|19<<5, returnInstruction)
	}

	function()
	helper := here()
	text = append(text, branchWithLink(here(), fakeStubsAddress+objcStubSize), returnInstruction)

	if f.hasStop {
		function()
		text = append(text, noOperation, noOperation)
		text = append(text, branchWithLink(here(), helper))
		text = append(text, testBit0OfW0(here(), here()-12), returnInstruction)
	}
	function()
	text = append(text, branchWithLink(here(), helper), noOperation, returnInstruction)

	stubs := make([]byte, 2*objcStubSize)
	binary.LittleEndian.PutUint32(stubs[objcStubSize:], adrp(1, fakeStubsAddress+objcStubSize, fakeSelrefsAddress))
	binary.LittleEndian.PutUint32(stubs[objcStubSize+4:], 0xf9400000|1<<10|1<<5|1)
	selrefs := make([]byte, 16)
	binary.LittleEndian.PutUint64(selrefs, fakeSelectorsAddress)
	binary.LittleEndian.PutUint64(selrefs[8:], terminateName)

	var functionStarts []byte
	previous := uint64(0)
	for _, start := range starts {
		functionStarts = binary.AppendUvarint(functionStarts, start-previous)
		previous = start
	}

	file := make([]byte, fakeSelrefsAddress+0x1000)
	for index, word := range text {
		binary.LittleEndian.PutUint32(file[fakeTextAddress+index*4:], word)
	}
	copy(file[fakeStringsAddress:], strings)
	copy(file[fakeSelectorsAddress:], selectors)
	copy(file[fakeStubsAddress:], stubs)
	copy(file[fakeSelrefsAddress:], selrefs)
	const functionStartsOffset = 0x3000
	copy(file[functionStartsOffset:], functionStarts)

	var commands bytes.Buffer
	segment := func(name string, sections ...[4]any) {
		binary.Write(&commands, binary.LittleEndian, uint32(loadSegment64))
		binary.Write(&commands, binary.LittleEndian, uint32(72+80*len(sections)))
		commands.Write(paddedName(name))
		binary.Write(&commands, binary.LittleEndian, [4]uint64{0, uint64(len(file)), 0, uint64(len(file))})
		binary.Write(&commands, binary.LittleEndian, [4]uint32{5, 5, uint32(len(sections)), 0})
		for _, section := range sections {
			commands.Write(paddedName(section[0].(string)))
			commands.Write(paddedName(name))
			binary.Write(&commands, binary.LittleEndian, [2]uint64{uint64(section[1].(int)), uint64(section[2].(int))})
			binary.Write(&commands, binary.LittleEndian, [8]uint32{uint32(section[1].(int))})
		}
	}
	segment("__TEXT",
		[4]any{"__text", fakeTextAddress, len(text) * 4},
		[4]any{"__cstring", fakeStringsAddress, len(strings)},
		[4]any{"__objc_methname", fakeSelectorsAddress, len(selectors)},
		[4]any{"__objc_stubs", fakeStubsAddress, len(stubs)},
	)
	segment("__DATA", [4]any{"__objc_selrefs", fakeSelrefsAddress, len(selrefs)})
	binary.Write(&commands, binary.LittleEndian, [2]uint32{loadUUID, 24})
	commands.Write([]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	binary.Write(&commands, binary.LittleEndian, [4]uint32{loadFunctionStarts, 16, functionStartsOffset, uint32(len(functionStarts))})

	binary.LittleEndian.PutUint32(file, machoMagic64)
	binary.LittleEndian.PutUint32(file[4:], cpuTypeArm64)
	binary.LittleEndian.PutUint32(file[16:], 4)
	binary.LittleEndian.PutUint32(file[20:], uint32(commands.Len()))
	copy(file[32:], commands.Bytes())
	return file
}

func paddedName(name string) []byte {
	padded := make([]byte, 16)
	copy(padded, name)
	return padded
}

func universal(arm64 []byte) []byte {
	const sliceOffset = 0x1000
	file := make([]byte, sliceOffset+len(arm64))
	binary.BigEndian.PutUint32(file, fatMagic)
	binary.BigEndian.PutUint32(file[4:], 2)
	binary.BigEndian.PutUint32(file[8:], 0x01000007)
	binary.BigEndian.PutUint32(file[28:], cpuTypeArm64)
	binary.BigEndian.PutUint32(file[36:], sliceOffset)
	binary.BigEndian.PutUint32(file[40:], uint32(len(arm64)))
	copy(file[sliceOffset:], arm64)
	return file
}

func TestFindClientCodeFindsBothInstructions(t *testing.T) {
	for name, file := range map[string][]byte{"arm64 only": wholeFakeClient.build(), "universal": universal(wholeFakeClient.build())} {
		code, err := FindClientCode(file)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if code.UUID != fakeUUID {
			t.Errorf("%s: UUID = %s, want %s", name, code.UUID, fakeUUID)
		}
		wantEnabled := Instruction{Address: fakeTextAddress + 10*4, Original: 0x1a9f17e8, Patched: 0x52800028}
		if code.Enabled != wantEnabled {
			t.Errorf("%s: enabled = %#x, want %#x", name, code.Enabled, wantEnabled)
		}
		stopAddress := uint64(fakeTextAddress + 18*4)
		wantStop := Instruction{Address: stopAddress, Original: testBit0OfW0(stopAddress, stopAddress-12), Patched: 0x14000000 | 0x3fffffd}
		if code.Stop != wantStop {
			t.Errorf("%s: stop = %#x, want %#x", name, code.Stop, wantStop)
		}
	}
}

func TestFindClientCodeRefusesWhatItDoesNotRecognise(t *testing.T) {
	cases := map[string][]byte{
		"two places set the flag":    fakeClient{enabledSites: 2, hasStop: true, hasProfileName: true}.build(),
		"no place sets the flag":     fakeClient{enabledSites: 0, hasStop: true, hasProfileName: true}.build(),
		"nothing gives up on a stop": fakeClient{enabledSites: 1, hasStop: false, hasProfileName: true}.build(),
		"no profile name":            fakeClient{enabledSites: 1, hasStop: true, hasProfileName: false}.build(),
		"not a Mach-O file":          []byte("fake steamclient"),
		"no arm64 half":              universal(wholeFakeClient.build())[:0x1000],
	}
	for name, file := range cases {
		code, err := FindClientCode(file)
		if failure.CodeOf(err) != failure.CodeUnknownClientCode {
			t.Errorf("%s: got %+v, %v; want %s", name, code, err, failure.CodeUnknownClientCode)
		}
	}
}
