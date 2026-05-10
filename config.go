package main

const (
	MAGIC uint32 = 0x00ea83f3
	MEMORY_SIZE = 4096
	REG_NUMBER = 16
	IDX_MOD = 512
	MAX_ARGS_NUMBER = 3

	PROG_NAME_LENGTH = 128
	COMMENT_LENGTH = 2048
	CHAMP_MAX_SIZE = 682

	CYCLE_TO_DIE = 1536
	CYCLE_DELTA = 50
	NBR_LIVE = 40
	MAX_CHECKS = 10

	REG_SIZE = 4
	DIR_SIZE = 4
	IND_SIZE = 2
	OPCODE_SIZE = 1
	PCODE_SIZE = 1
)

type OpCode int

const (
	OP_LIVE OpCode = iota + 1
	OP_LD
	OP_ST
	OP_ADD
	OP_SUB
	OP_AND
	OP_OR
	OP_XOR
	OP_ZJMP
	OP_LDI
	OP_STI
	OP_FORK
	OP_LLD
	OP_LLDI
	OP_LFORK
	OP_NOP
)

type ParamType int

const (
	PARAM_REG ParamType = iota
	PARAM_DIR
	PARAM_IND
)

type Instruction struct {
	Name     string
	OpCode   OpCode
	NbParams int
	Cycles   int
	HasPcode bool
	HasIdx   bool
	Params   []ParamType
}

var Instructions = map[string]Instruction{
	"live": {Name: "live", OpCode: OP_LIVE, NbParams: 1, Cycles: 10, HasPcode: false, HasIdx: false, Params: []ParamType{PARAM_DIR}},
	"ld":   {Name: "ld", OpCode: OP_LD, NbParams: 2, Cycles: 5, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_IND, PARAM_DIR}},
	"st":   {Name: "st", OpCode: OP_ST, NbParams: 2, Cycles: 5, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_REG, PARAM_IND}},
	"add":  {Name: "add", OpCode: OP_ADD, NbParams: 3, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_REG, PARAM_REG, PARAM_REG}},
	"sub":  {Name: "sub", OpCode: OP_SUB, NbParams: 3, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_REG, PARAM_REG, PARAM_REG}},
	"and":  {Name: "and", OpCode: OP_AND, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_IND, PARAM_IND, PARAM_REG}},
	"or":   {Name: "or", OpCode: OP_OR, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_IND, PARAM_IND, PARAM_REG}},
	"xor":  {Name: "xor", OpCode: OP_XOR, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_IND, PARAM_IND, PARAM_REG}},
	"zjmp": {Name: "zjmp", OpCode: OP_ZJMP, NbParams: 1, Cycles: 20, HasPcode: false, HasIdx: true, Params: []ParamType{PARAM_DIR}},
	"ldi":  {Name: "ldi", OpCode: OP_LDI, NbParams: 3, Cycles: 25, HasPcode: true, HasIdx: true, Params: []ParamType{PARAM_IND, PARAM_DIR, PARAM_REG}},
	"sti":  {Name: "sti", OpCode: OP_STI, NbParams: 3, Cycles: 25, HasPcode: true, HasIdx: true, Params: []ParamType{PARAM_REG, PARAM_IND, PARAM_REG}},
	"fork": {Name: "fork", OpCode: OP_FORK, NbParams: 1, Cycles: 800, HasPcode: false, HasIdx: true, Params: []ParamType{PARAM_DIR}},
	"lld":  {Name: "lld", OpCode: OP_LLD, NbParams: 2, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_IND, PARAM_DIR}},
	"lldi": {Name: "lldi", OpCode: OP_LLDI, NbParams: 3, Cycles: 50, HasPcode: true, HasIdx: true, Params: []ParamType{PARAM_IND, PARAM_DIR, PARAM_REG}},
	"lfork":{Name: "lfork", OpCode: OP_LFORK, NbParams: 1, Cycles: 1000, HasPcode: false, HasIdx: true, Params: []ParamType{PARAM_DIR}},
	"nop":  {Name: "nop", OpCode: OP_NOP, NbParams: 1, Cycles: 2, HasPcode: true, HasIdx: false, Params: []ParamType{PARAM_REG}},
}