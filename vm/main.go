package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
)

const MEM = 4096
const CTD = 1536
const CDELTA = 50

type Champ struct {
	Name, Desc string
	Code       []byte
	Id         int
}

type Proc struct {
	PC    int
	Reg   [17]int32
	Carry bool
	Last  int
	Id    int
}

func main() {
	dump := flag.Int("d", 0, "dump")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage: vm [-d N] <file.cor>...")
		os.Exit(1)
	}

	vm := &VM{Mem: make([]byte, MEM), Procs: make([]*Proc, 0)}
	for i, f := range flag.Args() {
		c, err := loadChamp(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		c.Id = i + 1
		vm.Champs = append(vm.Champs, c)
	}

	if len(vm.Champs) == 0 {
		os.Exit(1)
	}
	vm.run(*dump)
}

func loadChamp(path string) (*Champ, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var magic uint32
	binary.Read(f, binary.BigEndian, &magic)
	if magic != 0x00ea83f3 {
		return nil, errors.New("bad magic")
	}

	name := make([]byte, 128)
	f.Read(name)
	ni := 0
	for ; ni < 128 && name[ni] != 0; ni++ {
	}

	var size uint32
	binary.Read(f, binary.BigEndian, &size)

	desc := make([]byte, 2048)
	f.Read(desc)
	di := 0
	for ; di < 2048 && desc[di] != 0; di++ {
	}

	code := make([]byte, size)
	f.Read(code)

	return &Champ{Name: string(name[:ni]), Desc: string(desc[:di]), Code: code}, nil
}

type VM struct {
	Mem    []byte
	Champs []*Champ
	Procs  []*Proc
	Cycle  int
	Ctd    int
	Lives  int
	Checks int
	DumpAt int
}

func (vm *VM) run(dump int) {
	vm.DumpAt = dump
	vm.Ctd = CTD

	fmt.Println("For this match the players will be:")
	for i, c := range vm.Champs {
		fmt.Printf("Player %d (%d bytes): %s (%s)\n", i+1, len(c.Code), c.Name, c.Desc)
	}

	space := MEM / len(vm.Champs)
	for i, c := range vm.Champs {
		pos := i * space
		copy(vm.Mem[pos:pos+len(c.Code)], c.Code)
		p := &Proc{PC: pos, Last: 0, Id: c.Id}
		p.Reg[1] = -int32(c.Id)
		vm.Procs = append(vm.Procs, p)
	}

	for len(vm.Procs) > 0 {
		if vm.DumpAt > 0 && vm.Cycle >= vm.DumpAt {
			vm.dump()
			return
		}

		old := vm.Procs
		vm.Procs = nil
		for _, p := range old {
			vm.exec(p)
			if p.Last >= 0 {
				vm.Procs = append(vm.Procs, p)
			}
		}

		vm.Cycle++

		if vm.Cycle%50 == 0 || vm.Lives >= 40 {
			vm.check()
		}
	}

	if vm.Cycle <= 0 {
		fmt.Printf("cycle %d: Nobody wins!.\n", CTD)
		return
	}
	fmt.Printf("cycle %d: Nobody wins!.\n", vm.Cycle)
}

var cycles = []int{0, 10, 5, 5, 10, 10, 6, 6, 6, 20, 25, 25, 800, 10, 50, 1000, 2}

func (vm *VM) exec(p *Proc) {
	op := int(vm.Mem[p.PC])
	p.PC = (p.PC + 1) % MEM
	if op < 1 || op > 16 {
		return
	}

	vm.Cycle += cycles[op] - 1

	args := vm.readArgs(p, op)
	switch op {
	case 1: // live
		p.Last = vm.Cycle
		vm.Lives++

	case 2: // ld
		if len(args) >= 2 {
			addr := (p.PC + int(args[0])%512) % MEM
			p.Reg[args[1]] = vm.rd4(addr)
			p.Carry = p.Reg[args[1]] == 0
		}

	case 3: // st
		if len(args) >= 2 {
			addr := (p.PC + int(args[1])%512) % MEM
			vm.wr4(addr, p.Reg[args[0]])
		}

	case 4: // add
		if len(args) >= 3 {
			p.Reg[args[2]] = p.Reg[args[0]] + p.Reg[args[1]]
			p.Carry = p.Reg[args[2]] == 0
		}

	case 5: // sub
		if len(args) >= 3 {
			p.Reg[args[2]] = p.Reg[args[0]] - p.Reg[args[1]]
			p.Carry = p.Reg[args[2]] == 0
		}

	case 6: // and
		if len(args) >= 3 {
			p.Reg[args[2]] = vm.getArg(p, args[0]) & vm.getArg(p, args[1])
			p.Carry = p.Reg[args[2]] == 0
		}

	case 7: // or
		if len(args) >= 3 {
			p.Reg[args[2]] = vm.getArg(p, args[0]) | vm.getArg(p, args[1])
			p.Carry = p.Reg[args[2]] == 0
		}

	case 8: // xor
		if len(args) >= 3 {
			p.Reg[args[2]] = vm.getArg(p, args[0]) ^ vm.getArg(p, args[1])
			p.Carry = p.Reg[args[2]] == 0
		}

	case 9: // zjmp
		if len(args) >= 1 && p.Carry {
			p.PC = (p.PC + int(args[0])%512) % MEM
		}

	case 10: // ldi
		if len(args) >= 3 {
			addr := (p.PC + int(vm.getArg(p, args[0])+vm.getArg(p, args[1]))%512) % MEM
			p.Reg[args[2]] = vm.rd4(addr)
			p.Carry = p.Reg[args[2]] == 0
		}

	case 11: // sti
		if len(args) >= 3 {
			val := p.Reg[args[0]]
			addr := (p.PC + int(vm.getArg(p, args[1])+vm.getArg(p, args[2]))%512) % MEM
			vm.wr4(addr, val)
		}

	case 12: // fork
		if len(args) >= 1 {
			np := &Proc{PC: (p.PC + int(args[0])%512) % MEM, Reg: p.Reg, Carry: p.Carry, Last: p.Last, Id: p.Id}
			vm.Procs = append(vm.Procs, np)
		}

	case 13: // lld
		if len(args) >= 2 {
			addr := (p.PC + int(args[0])) % MEM
			p.Reg[args[1]] = vm.rd4(addr)
			p.Carry = p.Reg[args[1]] == 0
		}

	case 14: // lldi
		if len(args) >= 3 {
			addr := (p.PC + int(vm.getArg(p, args[0])+vm.getArg(p, args[1]))) % MEM
			p.Reg[args[2]] = vm.rd4(addr)
			p.Carry = p.Reg[args[2]] == 0
		}

	case 15: // lfork
		if len(args) >= 1 {
			np := &Proc{PC: (p.PC + int(args[0])) % MEM, Reg: p.Reg, Carry: p.Carry, Last: p.Last, Id: p.Id}
			vm.Procs = append(vm.Procs, np)
		}
	}
}

func (vm *VM) readArgs(p *Proc, op int) []int32 {
	// opcodes with no pcode: live(1), zjmp(9), fork(12), lfork(15), nop(16)
	noPcode := map[int]bool{1: true, 9: true, 12: true, 15: true, 16: true}
	// opcodes with 2-byte direct: zjmp, ldi, sti, fork, lldi, lfork
	shortDir := map[int]bool{9: true, 10: true, 11: true, 12: true, 14: true, 15: true}

	if noPcode[op] {
		if op == 16 { // nop
			if p.PC < MEM {
				p.PC = (p.PC + 1) % MEM
			}
			return nil
		}
		// Read one arg
		if shortDir[op] {
			if p.PC+1 < MEM {
				v := int16(vm.Mem[p.PC])<<8 | int16(vm.Mem[p.PC+1])
				p.PC = (p.PC + 2) % MEM
				return []int32{int32(v)}
			}
		} else {
			if p.PC+3 < MEM {
				v := int32(vm.Mem[p.PC])<<24 | int32(vm.Mem[p.PC+1])<<16 | int32(vm.Mem[p.PC+2])<<8 | int32(vm.Mem[p.PC+3])
				p.PC = (p.PC + 4) % MEM
				return []int32{v}
			}
		}
		return nil
	}

	// Has pcode
	if p.PC >= MEM {
		return nil
	}
	pcode := vm.Mem[p.PC]
	p.PC = (p.PC + 1) % MEM

	var args []int32
	for i := 0; i < 3; i++ {
		if p.PC >= MEM {
			break
		}
		// Get param type from pcode bits 6-i*2
		t := (pcode >> (6 - i*2)) & 3

		if t == 1 { // register
			args = append(args, int32(vm.Mem[p.PC]))
			p.PC = (p.PC + 1) % MEM
		} else if shortDir[op] { // 2-byte
			if p.PC+1 < MEM {
				v := int16(vm.Mem[p.PC])<<8 | int16(vm.Mem[p.PC+1])
				p.PC = (p.PC + 2) % MEM
				args = append(args, int32(v))
			}
		} else { // 4-byte
			if p.PC+3 < MEM {
				v := int32(vm.Mem[p.PC])<<24 | int32(vm.Mem[p.PC+1])<<16 | int32(vm.Mem[p.PC+2])<<8 | int32(vm.Mem[p.PC+3])
				p.PC = (p.PC + 4) % MEM
				args = append(args, v)
			}
		}
	}
	return args
}

func (vm *VM) getArg(p *Proc, v int32) int32 {
	if v >= 1 && v <= 16 {
		return p.Reg[v]
	}
	addr := (p.PC + int(v)%512) % MEM
	return vm.rd4(addr)
}

func (vm *VM) rd4(a int) int32 {
	a = a % MEM
	return int32(vm.Mem[a])<<24 | int32(vm.Mem[(a+1)%MEM])<<16 | int32(vm.Mem[(a+2)%MEM])<<8 | int32(vm.Mem[(a+3)%MEM])
}

func (vm *VM) wr4(a int, v int32) {
	a = a % MEM
	vm.Mem[a] = byte((v >> 24) & 0xFF)
	vm.Mem[(a+1)%MEM] = byte((v >> 16) & 0xFF)
	vm.Mem[(a+2)%MEM] = byte((v >> 8) & 0xFF)
	vm.Mem[(a+3)%MEM] = byte(v & 0xFF)
}

func (vm *VM) check() {
	var alive []*Proc
	for _, p := range vm.Procs {
		if vm.Cycle-p.Last <= vm.Ctd {
			alive = append(alive, p)
		}
	}
	vm.Procs = alive
	vm.Lives = 0
	vm.Checks++

	if vm.Lives >= 40 || vm.Checks >= 10 {
		vm.Ctd -= CDELTA
		vm.Checks = 0
		if vm.Ctd <= 0 {
			vm.Procs = nil
		}
	}
}

func (vm *VM) dump() {
	for i := 0; i < MEM; i += 32 {
		fmt.Printf("%04x ", i)
		for j := 0; j < 32; j++ {
			fmt.Printf("%02x ", vm.Mem[i+j])
		}
		fmt.Println()
	}
}