package steamhook

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"slices"

	"github.com/ThallesP/macron/internal/failure"
)

type ClientCode struct {
	UUID    string
	Enabled Instruction
	Stop    Instruction
}

type Instruction struct {
	Address  uint64
	Original uint32
	Patched  uint32
}

const (
	enabledProfileName = "CCompatManager::BIsCompatibilityToolEnabled"
	linuxPlatformName  = "linux"
	terminateSelector  = "terminate"

	lowestEnabledFlagOffset = 0x100
	stringLoadWindow        = 8
)

// FindClientCode locates, in the arm64 half of a steamclient.dylib, the two
// instructions the Steam Play switch changes (steamplay/README.md says what
// each one does). It refuses a binary in which either is missing or ambiguous.
func FindClientCode(steamclient []byte) (ClientCode, error) {
	image, err := readArm64Image(steamclient)
	if err != nil {
		return ClientCode{}, err
	}

	enabled, err := image.enabledInstruction()
	if err != nil {
		return ClientCode{}, err
	}
	stop, err := image.stopInstruction()
	if err != nil {
		return ClientCode{}, err
	}
	return ClientCode{UUID: image.uuid, Enabled: enabled, Stop: stop}, nil
}

type section struct {
	address uint64
	data    []byte
}

func (s section) contains(address uint64) bool {
	return address >= s.address && address < s.address+uint64(len(s.data))
}

type arm64Image struct {
	uuid           string
	text           section
	strings        section
	selectorNames  section
	selectorRefs   section
	selectorStubs  section
	functionStarts []uint64
}

func (i arm64Image) instruction(address uint64) uint32 {
	return binary.LittleEndian.Uint32(i.text.data[address-i.text.address:])
}

func (i arm64Image) instructionCount() uint64 {
	return uint64(len(i.text.data)) / 4
}

func (i arm64Image) functionAround(address uint64) (start uint64, end uint64) {
	index, isStart := slices.BinarySearch(i.functionStarts, address)
	if !isStart {
		index--
	}
	if index < 0 {
		return i.text.address, i.text.address
	}
	if index+1 == len(i.functionStarts) {
		return i.functionStarts[index], i.text.address + uint64(len(i.text.data))
	}
	return i.functionStarts[index], i.functionStarts[index+1]
}

func stringAddress(strings section, text string) (uint64, bool) {
	needle := append([]byte(text), 0)
	for from := 0; ; {
		found := bytes.Index(strings.data[from:], needle)
		if found < 0 {
			return 0, false
		}
		at := from + found
		if at == 0 || strings.data[at-1] == 0 {
			return strings.address + uint64(at), true
		}
		from = at + 1
	}
}

// Addresses of the add that completes an adrp/add pair forming `target`.
func (i arm64Image) loadsOf(target uint64) []uint64 {
	var loads []uint64
	pageOf := map[uint32]uint64{}
	setAt := map[uint32]uint64{}
	for index := range i.instructionCount() {
		address := i.text.address + index*4
		word := i.instruction(address)
		if page, register, isAdrp := decodeAdrp(word, address); isAdrp {
			pageOf[register] = page
			setAt[register] = index
			continue
		}
		offset, base, isAdd := decodeAddImmediate(word)
		if !isAdd {
			continue
		}
		page, isLoaded := pageOf[base]
		if isLoaded && index-setAt[base] <= stringLoadWindow && page+offset == target {
			loads = append(loads, address)
		}
	}
	return loads
}

func (i arm64Image) enabledFlagOffset() (uint32, error) {
	name, hasName := stringAddress(i.strings, enabledProfileName)
	if !hasName {
		return 0, clientCodeError("it has no %q string", enabledProfileName)
	}
	loads := i.loadsOf(name)
	if len(loads) != 1 {
		return 0, clientCodeError("%q is used in %d places, want 1", enabledProfileName, len(loads))
	}

	_, end := i.functionAround(loads[0])
	for address := loads[0]; address < end; address += 4 {
		offset, isLoad := decodeLoadByte(i.instruction(address))
		if isLoad && offset >= lowestEnabledFlagOffset {
			return offset, nil
		}
	}
	return 0, clientCodeError("the function that uses %q never reads the enabled flag", enabledProfileName)
}

// The compat manager's constructor ends its platform check with
//
//	bl V_strnicmp ; cmp w0, #0 ; cset wR, eq ; strb wR, [xN, #flag]
//
// after loading the "linux" string. The cset becomes mov wR, #1.
func (i arm64Image) enabledInstruction() (Instruction, error) {
	flag, err := i.enabledFlagOffset()
	if err != nil {
		return Instruction{}, err
	}
	linux, hasLinux := stringAddress(i.strings, linuxPlatformName)
	if !hasLinux {
		return Instruction{}, clientCodeError("it has no %q string", linuxPlatformName)
	}

	var found []Instruction
	for _, load := range i.loadsOf(linux) {
		for address := load; address < load+stringLoadWindow*4 && address+8 < i.text.address+uint64(len(i.text.data)); address += 4 {
			register, isSetEqual := decodeSetEqual(i.instruction(address + 4))
			if i.instruction(address) != compareW0WithZero || !isSetEqual {
				continue
			}
			if i.instruction(address+8) != storeByte(register, flag)|i.instruction(address+8)&baseRegisterMask {
				continue
			}
			found = append(found, Instruction{Address: address + 4, Original: i.instruction(address + 4), Patched: moveOne(register)})
		}
	}
	if len(found) != 1 {
		return Instruction{}, clientCodeError("%d places set the enabled flag (offset %#x) from the platform name, want 1", len(found), flag)
	}
	return found[0], nil
}

// Steam stops a game with -[NSRunningApplication terminate] and gives up when
// the process is not an application:
//
//	bl terminate_helper ; tbnz w0, #0, carry_on
//
// The tbnz becomes b carry_on, so Steam goes on to its signals.
func (i arm64Image) stopInstruction() (Instruction, error) {
	stub, err := i.selectorStub(terminateSelector)
	if err != nil {
		return Instruction{}, err
	}
	stubCallers := i.callersOf(stub)
	if len(stubCallers) != 1 {
		return Instruction{}, clientCodeError("%d places send %q, want 1", len(stubCallers), terminateSelector)
	}
	helper, _ := i.functionAround(stubCallers[0])

	var found []Instruction
	for _, caller := range i.callersOf(helper) {
		next := caller + 4
		if !i.text.contains(next) {
			continue
		}
		target, isTest := decodeTestBit0OfW0(i.instruction(next), next)
		if !isTest {
			continue
		}
		found = append(found, Instruction{Address: next, Original: i.instruction(next), Patched: branch(next, target)})
	}
	if len(found) != 1 {
		return Instruction{}, clientCodeError("%d places give up when %q fails, want 1", len(found), terminateSelector)
	}
	return found[0], nil
}

func (i arm64Image) selectorStub(selector string) (uint64, error) {
	name, hasName := stringAddress(i.selectorNames, selector)
	if !hasName {
		return 0, clientCodeError("it has no %q selector", selector)
	}

	reference := uint64(0)
	for offset := 0; offset+8 <= len(i.selectorRefs.data); offset += 8 {
		if chainedPointerTarget(binary.LittleEndian.Uint64(i.selectorRefs.data[offset:])) == name {
			reference = i.selectorRefs.address + uint64(offset)
		}
	}
	if reference == 0 {
		return 0, clientCodeError("nothing refers to the %q selector", selector)
	}

	for offset := 0; offset+8 <= len(i.selectorStubs.data); offset += objcStubSize {
		address := i.selectorStubs.address + uint64(offset)
		page, _, isAdrp := decodeAdrp(binary.LittleEndian.Uint32(i.selectorStubs.data[offset:]), address)
		pageOffset, isLoad := decodeLoadPointer(binary.LittleEndian.Uint32(i.selectorStubs.data[offset+4:]))
		if isAdrp && isLoad && page+pageOffset == reference {
			return address, nil
		}
	}
	return 0, clientCodeError("no stub sends the %q selector", selector)
}

func (i arm64Image) callersOf(target uint64) []uint64 {
	var callers []uint64
	for index := range i.instructionCount() {
		address := i.text.address + index*4
		if destination, isCall := decodeBranchWithLink(i.instruction(address), address); isCall && destination == target {
			callers = append(callers, address)
		}
	}
	return callers
}

func clientCodeError(format string, args ...any) *failure.Error {
	return failure.Errorf(failure.CodeUnknownClientCode, "steamclient.dylib is not laid out the way the Steam Play switch expects: "+format, args...)
}

const (
	machoMagic64       = 0xfeedfacf
	fatMagic           = 0xcafebabe
	cpuTypeArm64       = 0x0100000c
	loadSegment64      = 0x19
	loadUUID           = 0x1b
	loadFunctionStarts = 0x26
	objcStubSize       = 0x20
)

func readArm64Image(file []byte) (arm64Image, error) {
	slice, err := arm64Slice(file)
	if err != nil {
		return arm64Image{}, err
	}
	if len(slice) < 32 || binary.LittleEndian.Uint32(slice) != machoMagic64 {
		return arm64Image{}, clientCodeError("its arm64 half is not a 64-bit Mach-O file")
	}

	image := arm64Image{}
	textBase := uint64(0)
	commandCount := binary.LittleEndian.Uint32(slice[16:])
	offset := 32
	for range commandCount {
		if offset+8 > len(slice) {
			return arm64Image{}, clientCodeError("its load commands run past the end of the file")
		}
		command := binary.LittleEndian.Uint32(slice[offset:])
		size := int(binary.LittleEndian.Uint32(slice[offset+4:]))
		if size < 8 || offset+size > len(slice) {
			return arm64Image{}, clientCodeError("its load commands run past the end of the file")
		}
		body := slice[offset : offset+size]

		switch command {
		case loadUUID:
			image.uuid = hex.EncodeToString(body[8:24])
		case loadSegment64:
			if cString(body[8:24]) == "__TEXT" {
				textBase = binary.LittleEndian.Uint64(body[24:])
			}
			if err := image.readSections(slice, body); err != nil {
				return arm64Image{}, err
			}
		case loadFunctionStarts:
			dataOffset := binary.LittleEndian.Uint32(body[8:])
			dataSize := binary.LittleEndian.Uint32(body[12:])
			if uint64(dataOffset)+uint64(dataSize) > uint64(len(slice)) {
				return arm64Image{}, clientCodeError("its function starts run past the end of the file")
			}
			image.functionStarts = decodeFunctionStarts(slice[dataOffset:dataOffset+dataSize], textBase)
		default:
		}
		offset += size
	}

	if image.uuid == "" || len(image.text.data) == 0 || len(image.strings.data) == 0 || len(image.functionStarts) == 0 {
		return arm64Image{}, clientCodeError("it lacks a build id, code, strings or function starts")
	}
	return image, nil
}

func (i *arm64Image) readSections(slice []byte, segment []byte) error {
	sectionCount := binary.LittleEndian.Uint32(segment[64:])
	for index := range int(sectionCount) {
		header := segment[72+index*80:]
		address := binary.LittleEndian.Uint64(header[32:])
		size := binary.LittleEndian.Uint64(header[40:])
		fileOffset := uint64(binary.LittleEndian.Uint32(header[48:]))

		destination, isWanted := map[string]*section{
			"__TEXT.__text":               &i.text,
			"__TEXT.__cstring":            &i.strings,
			"__TEXT.__objc_methname":      &i.selectorNames,
			"__TEXT.__objc_stubs":         &i.selectorStubs,
			"__DATA.__objc_selrefs":       &i.selectorRefs,
			"__DATA_CONST.__objc_selrefs": &i.selectorRefs,
		}[cString(header[16:32])+"."+cString(header[0:16])]
		if !isWanted {
			continue
		}
		if fileOffset+size > uint64(len(slice)) {
			return clientCodeError("section %s runs past the end of the file", cString(header[0:16]))
		}
		*destination = section{address: address, data: slice[fileOffset : fileOffset+size]}
	}
	return nil
}

func arm64Slice(file []byte) ([]byte, error) {
	if len(file) < 8 {
		return nil, clientCodeError("it is too short to be a Mach-O file")
	}
	if binary.LittleEndian.Uint32(file) == machoMagic64 {
		return file, nil
	}
	if binary.BigEndian.Uint32(file) != fatMagic {
		return nil, clientCodeError("it is not a Mach-O file")
	}

	architectureCount := int(binary.BigEndian.Uint32(file[4:]))
	for index := range architectureCount {
		entry := 8 + index*20
		if entry+20 > len(file) {
			break
		}
		offset := uint64(binary.BigEndian.Uint32(file[entry+8:]))
		size := uint64(binary.BigEndian.Uint32(file[entry+12:]))
		if binary.BigEndian.Uint32(file[entry:]) != cpuTypeArm64 {
			continue
		}
		if offset+size > uint64(len(file)) {
			return nil, clientCodeError("its arm64 half runs past the end of the file")
		}
		return file[offset : offset+size], nil
	}
	return nil, clientCodeError("it has no arm64 half")
}

func decodeFunctionStarts(data []byte, textBase uint64) []uint64 {
	var starts []uint64
	address := textBase
	for position := 0; position < len(data); {
		delta, length := binary.Uvarint(data[position:])
		if length <= 0 || delta == 0 {
			break
		}
		position += length
		address += delta
		starts = append(starts, address)
	}
	return starts
}

func cString(field []byte) string {
	name, _, _ := bytes.Cut(field, []byte{0})
	return string(name)
}

// The low 36 bits of a 64-bit chained fixup hold the target (an address, or an
// offset from an image based at zero, which is the same number in a dylib).
func chainedPointerTarget(pointer uint64) uint64 {
	return pointer & 0xfffffffff
}

const (
	compareW0WithZero = 0x7100001f
	baseRegisterMask  = 0x1f << 5
)

func decodeAdrp(word uint32, address uint64) (page uint64, register uint32, isAdrp bool) {
	if word&0x9f000000 != 0x90000000 {
		return 0, 0, false
	}
	immediate := int64(word>>5&0x7ffff)<<2 | int64(word>>29&3)
	if immediate&(1<<20) != 0 {
		immediate -= 1 << 21
	}
	return uint64(int64(address&^0xfff) + immediate<<12), word & 0x1f, true
}

func decodeAddImmediate(word uint32) (immediate uint64, base uint32, isAdd bool) {
	if word&0xffc00000 != 0x91000000 {
		return 0, 0, false
	}
	return uint64(word >> 10 & 0xfff), word >> 5 & 0x1f, true
}

func decodeLoadByte(word uint32) (offset uint32, isLoad bool) {
	return word >> 10 & 0xfff, word&0xffc00000 == 0x39400000
}

func decodeLoadPointer(word uint32) (offset uint64, isLoad bool) {
	return uint64(word>>10&0xfff) * 8, word&0xffc00000 == 0xf9400000
}

func decodeSetEqual(word uint32) (register uint32, isSetEqual bool) {
	return word & 0x1f, word&^0x1f == 0x1a9f17e0
}

func storeByte(register uint32, offset uint32) uint32 {
	return 0x39000000 | offset<<10 | register
}

func moveOne(register uint32) uint32 {
	return 0x52800020 | register
}

func decodeBranchWithLink(word uint32, address uint64) (target uint64, isCall bool) {
	if word&0xfc000000 != 0x94000000 {
		return 0, false
	}
	immediate := int64(word & 0x3ffffff)
	if immediate&(1<<25) != 0 {
		immediate -= 1 << 26
	}
	return uint64(int64(address) + immediate*4), true
}

func decodeTestBit0OfW0(word uint32, address uint64) (target uint64, isTest bool) {
	if word&0xfff8001f != 0x37000000 {
		return 0, false
	}
	immediate := int64(word >> 5 & 0x3fff)
	if immediate&(1<<13) != 0 {
		immediate -= 1 << 14
	}
	return uint64(int64(address) + immediate*4), true
}

func branch(from uint64, to uint64) uint32 {
	return 0x14000000 | uint32((int64(to)-int64(from))/4)&0x3ffffff
}
