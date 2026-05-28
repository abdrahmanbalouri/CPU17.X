package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type Champ struct {
	Name, Desc string
	Code       []byte
	Id         int
	LastLive   int
}

type Proc struct {
	PC    int
	Reg   [REG_NUMBER + 1]int32
	Carry bool
	Last  int
	Id    int
}

type Arg struct {
	Type ParamType
	Val  int32
}

func main() {
	dump := flag.Int("d", -1, "dump memory at cycle N")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Corewar virtual machine")
		fmt.Println("Usage: vm [-d N] <file.cor> [file.cor ...]")
		os.Exit(1)
	}
	if flag.NArg() > 4 {
		fmt.Fprintln(os.Stderr, "Error: at most 4 players are allowed")
		os.Exit(1)
	}

	vm := &VM{Mem: make([]byte, MEMORY_SIZE)}
	for i, path := range flag.Args() {
		champ, err := loadChamp(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		champ.Id = i + 1
		vm.Champs = append(vm.Champs, champ)
	}
	vm.run(*dump)
}

func loadChamp(path string) (*Champ, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var magic uint32
	if err := binary.Read(f, binary.BigEndian, &magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if magic != MAGIC {
		return nil, errors.New("bad magic number")
	}

	nameBuf := make([]byte, PROG_NAME_LENGTH)
	if _, err := io.ReadFull(f, nameBuf); err != nil {
		return nil, fmt.Errorf("read name: %w", err)
	}

	var size uint32
	if err := binary.Read(f, binary.BigEndian, &size); err != nil {
		return nil, fmt.Errorf("read size: %w", err)
	}
	if size > CHAMP_MAX_SIZE {
		return nil, fmt.Errorf("champion too large: %d bytes (max %d)", size, CHAMP_MAX_SIZE)
	}

	descBuf := make([]byte, COMMENT_LENGTH)
	if _, err := io.ReadFull(f, descBuf); err != nil {
		return nil, fmt.Errorf("read description: %w", err)
	}

	code := make([]byte, size)
	if _, err := io.ReadFull(f, code); err != nil {
		return nil, fmt.Errorf("read code: %w", err)
	}

	return &Champ{Name: cstring(nameBuf), Desc: cstring(descBuf), Code: code}, nil
}

func cstring(b []byte) string {
	for i, v := range b {
		if v == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

type VM struct {
	Mem    []byte
	Champs []*Champ
	Procs  []*Proc
	Cycle  int
	Ctd    int
	Lives  int
	Checks int
}

func (vm *VM) run(dumpAt int) {
	vm.Ctd = CYCLE_TO_DIE

	fmt.Println("Introducing contestants...")
	for _, champ := range vm.Champs {
		fmt.Printf("* Player %d, weighing %d bytes, \"%s\" (\"%s\")\n", champ.Id, len(champ.Code), champ.Name, champ.Desc)
	}

	space := MEMORY_SIZE / len(vm.Champs)
	for i, champ := range vm.Champs {
		pos := i * space
		copy(vm.Mem[pos:pos+len(champ.Code)], champ.Code)
		proc := &Proc{PC: pos, Id: champ.Id}
		proc.Reg[1] = -int32(champ.Id)
		vm.Procs = append(vm.Procs, proc)
	}

	for len(vm.Procs) > 0 {
		if dumpAt >= 0 && vm.Cycle == dumpAt {
			vm.dump()
			return
		}

		current := vm.Procs
		vm.Procs = nil
		for _, proc := range current {
			vm.exec(proc)
			if proc.Last >= 0 {
				vm.Procs = append(vm.Procs, proc)
			}
		}

		vm.Cycle++
		if vm.Cycle%vm.Ctd == 0 || vm.Lives >= NBR_LIVE {
			vm.check()
		}
	}

	vm.printWinner()
}

func (vm *VM) exec(p *Proc) {
	base := p.PC
	op := OpCode(vm.Mem[base])
	p.PC = wrap(base + 1)

	inst, ok := InstructionsByOp[op]
	if !ok {
		return
	}

	args, ok := vm.readArgs(p, inst)
	if !ok {
		return
	}

	switch op {
	case OP_LIVE:
		vm.live(p, int(args[0].Val))

	case OP_LD, OP_LLD:
		if args[1].Type == PARAM_REG {
			mod := op == OP_LD
			p.Reg[args[1].Val] = vm.value(p, base, args[0], mod)
			p.Carry = p.Reg[args[1].Val] == 0
		}

	case OP_ST:
		src := p.Reg[args[0].Val]
		if args[1].Type == PARAM_REG {
			p.Reg[args[1].Val] = src
		} else {
			vm.wr4(wrap(base+int(args[1].Val)%IDX_MOD), src)
		}

	case OP_ADD:
		p.Reg[args[2].Val] = p.Reg[args[0].Val] + p.Reg[args[1].Val]
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_SUB:
		p.Reg[args[2].Val] = p.Reg[args[0].Val] - p.Reg[args[1].Val]
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_AND:
		p.Reg[args[2].Val] = vm.value(p, base, args[0], true) & vm.value(p, base, args[1], true)
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_OR:
		p.Reg[args[2].Val] = vm.value(p, base, args[0], true) | vm.value(p, base, args[1], true)
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_XOR:
		p.Reg[args[2].Val] = vm.value(p, base, args[0], true) ^ vm.value(p, base, args[1], true)
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_ZJMP:
		if p.Carry {
			p.PC = wrap(base + int(args[0].Val)%IDX_MOD)
		}

	case OP_LDI, OP_LLDI:
		mod := op == OP_LDI
		a := vm.value(p, base, args[0], mod)
		b := vm.value(p, base, args[1], mod)
		addr := base + int(a+b)
		if mod {
			addr = base + int(a+b)%IDX_MOD
		}
		p.Reg[args[2].Val] = vm.rd4(addr)
		p.Carry = p.Reg[args[2].Val] == 0

	case OP_STI:
		a := vm.value(p, base, args[1], true)
		b := vm.value(p, base, args[2], true)
		vm.wr4(base+int(a+b)%IDX_MOD, p.Reg[args[0].Val])

	case OP_FORK, OP_LFORK:
		pc := base + int(args[0].Val)
		if op == OP_FORK {
			pc = base + int(args[0].Val)%IDX_MOD
		}
		clone := &Proc{PC: wrap(pc), Reg: p.Reg, Carry: p.Carry, Last: p.Last, Id: p.Id}
		vm.Procs = append(vm.Procs, clone)

	case OP_NOP:
	}
}

func (vm *VM) live(p *Proc, player int) {
	p.Last = vm.Cycle
	vm.Lives++
	for _, champ := range vm.Champs {
		if player == champ.Id || player == -champ.Id {
			champ.LastLive = vm.Cycle
			return
		}
	}
	if p.Id >= 1 && p.Id <= len(vm.Champs) {
		vm.Champs[p.Id-1].LastLive = vm.Cycle
	}
}

func (vm *VM) readArgs(p *Proc, inst Instruction) ([]Arg, bool) {
	args := make([]Arg, 0, inst.NbParams)
	if !inst.HasPcode {
		val := vm.readInt(p, directSize(inst))
		return []Arg{{Type: PARAM_DIR, Val: val}}, true
	}

	pcode := vm.Mem[p.PC]
	p.PC = wrap(p.PC + 1)
	for i := 0; i < inst.NbParams; i++ {
		typ := ParamType((pcode >> (6 - i*2)) & 3)
		if typ == 0 || inst.Params[i]&maskFor(typ) == 0 {
			return nil, false
		}
		val := vm.readInt(p, argumentSize(typ, inst.HasIdx))
		if typ == PARAM_REG && (val < 1 || val > REG_NUMBER) {
			return nil, false
		}
		args = append(args, Arg{Type: typ, Val: val})
	}
	return args, true
}

func directSize(inst Instruction) int {
	if inst.HasIdx {
		return IND_SIZE
	}
	return DIR_SIZE
}

func argumentSize(t ParamType, hasIdx bool) int {
	switch t {
	case PARAM_REG:
		return 1
	case PARAM_DIR:
		if hasIdx {
			return IND_SIZE
		}
		return DIR_SIZE
	default:
		return IND_SIZE
	}
}

func (vm *VM) readInt(p *Proc, size int) int32 {
	if size == 1 {
		val := int32(vm.Mem[p.PC])
		p.PC = wrap(p.PC + 1)
		return val
	}
	if size == 2 {
		val := int16(vm.Mem[p.PC])<<8 | int16(vm.Mem[wrap(p.PC+1)])
		p.PC = wrap(p.PC + 2)
		return int32(val)
	}
	val := vm.rd4(p.PC)
	p.PC = wrap(p.PC + 4)
	return val
}

func (vm *VM) value(p *Proc, base int, arg Arg, idxMod bool) int32 {
	switch arg.Type {
	case PARAM_REG:
		return p.Reg[arg.Val]
	case PARAM_DIR:
		return arg.Val
	default:
		addr := base + int(arg.Val)
		if idxMod {
			addr = base + int(arg.Val)%IDX_MOD
		}
		return vm.rd4(addr)
	}
}

func wrap(a int) int {
	a %= MEMORY_SIZE
	if a < 0 {
		a += MEMORY_SIZE
	}
	return a
}

func (vm *VM) rd4(a int) int32 {
	a = wrap(a)
	return int32(vm.Mem[a])<<24 | int32(vm.Mem[wrap(a+1)])<<16 | int32(vm.Mem[wrap(a+2)])<<8 | int32(vm.Mem[wrap(a+3)])
}

func (vm *VM) wr4(a int, v int32) {
	a = wrap(a)
	vm.Mem[a] = byte((v >> 24) & 0xFF)
	vm.Mem[wrap(a+1)] = byte((v >> 16) & 0xFF)
	vm.Mem[wrap(a+2)] = byte((v >> 8) & 0xFF)
	vm.Mem[wrap(a+3)] = byte(v & 0xFF)
}

func (vm *VM) check() {
	alive := vm.Procs[:0]
	for _, p := range vm.Procs {
		if vm.Cycle-p.Last <= vm.Ctd {
			alive = append(alive, p)
		}
	}
	vm.Procs = alive

	lives := vm.Lives
	vm.Lives = 0
	vm.Checks++
	if lives >= NBR_LIVE || vm.Checks >= MAX_CHECKS {
		vm.Ctd -= CYCLE_DELTA
		vm.Checks = 0
		if vm.Ctd <= 0 {
			vm.Procs = nil
		}
	}
}

func (vm *VM) printWinner() {
	var winner *Champ
	for _, champ := range vm.Champs {
		if winner == nil || champ.LastLive >= winner.LastLive {
			winner = champ
		}
	}
	if winner != nil {
		fmt.Printf("cycle %d: Player %d (%s) wins!\n", vm.Cycle, winner.Id, winner.Name)
	}
}

func (vm *VM) dump() {
	for i := 0; i < MEMORY_SIZE; i += 32 {
		fmt.Printf("%04x ", i)
		for j := 0; j < 32; j++ {
			fmt.Printf("%02x ", vm.Mem[i+j])
		}
		fmt.Println()
	}
}
