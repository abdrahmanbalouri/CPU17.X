package main

const (
	MAGIC           uint32 = 0x00ea83f3
	MEMORY_SIZE            = 4096
	REG_NUMBER             = 16
	IDX_MOD                = 512
	MAX_ARGS_NUMBER        = 3

	PROG_NAME_LENGTH = 128
	COMMENT_LENGTH   = 2048
	CHAMP_MAX_SIZE   = 682

	CYCLE_TO_DIE = 1536
	CYCLE_DELTA  = 50
	NBR_LIVE     = 40
	MAX_CHECKS   = 10

	REG_SIZE    = 4
	DIR_SIZE    = 4
	IND_SIZE    = 2
	OPCODE_SIZE = 1
	PCODE_SIZE  = 1
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

type ParamType byte

const (
	PARAM_REG ParamType = 1
	PARAM_DIR ParamType = 2
	PARAM_IND ParamType = 3
)

type ParamMask int

const (
	MASK_REG ParamMask = 1 << iota
	MASK_DIR
	MASK_IND
)

func maskFor(t ParamType) ParamMask {
	switch t {
	case PARAM_REG:
		return MASK_REG
	case PARAM_DIR:
		return MASK_DIR
	case PARAM_IND:
		return MASK_IND
	default:
		return 0
	}
}

type Instruction struct {
	Name     string
	OpCode   OpCode
	NbParams int
	Cycles   int
	HasPcode bool
	HasIdx   bool
	Params   []ParamMask
}

var Instructions = map[string]Instruction{
	"live":  {Name: "live", OpCode: OP_LIVE, NbParams: 1, Cycles: 10, HasPcode: false, HasIdx: false, Params: []ParamMask{MASK_DIR}},
	"ld":    {Name: "ld", OpCode: OP_LD, NbParams: 2, Cycles: 5, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_DIR | MASK_IND, MASK_REG}},
	"st":    {Name: "st", OpCode: OP_ST, NbParams: 2, Cycles: 5, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG, MASK_REG | MASK_IND}},
	"add":   {Name: "add", OpCode: OP_ADD, NbParams: 3, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG, MASK_REG, MASK_REG}},
	"sub":   {Name: "sub", OpCode: OP_SUB, NbParams: 3, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG, MASK_REG, MASK_REG}},
	"and":   {Name: "and", OpCode: OP_AND, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR | MASK_IND, MASK_REG}},
	"or":    {Name: "or", OpCode: OP_OR, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR | MASK_IND, MASK_REG}},
	"xor":   {Name: "xor", OpCode: OP_XOR, NbParams: 3, Cycles: 6, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR | MASK_IND, MASK_REG}},
	"zjmp":  {Name: "zjmp", OpCode: OP_ZJMP, NbParams: 1, Cycles: 20, HasPcode: false, HasIdx: true, Params: []ParamMask{MASK_DIR}},
	"ldi":   {Name: "ldi", OpCode: OP_LDI, NbParams: 3, Cycles: 25, HasPcode: true, HasIdx: true, Params: []ParamMask{MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR, MASK_REG}},
	"sti":   {Name: "sti", OpCode: OP_STI, NbParams: 3, Cycles: 25, HasPcode: true, HasIdx: true, Params: []ParamMask{MASK_REG, MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR}},
	"fork":  {Name: "fork", OpCode: OP_FORK, NbParams: 1, Cycles: 800, HasPcode: false, HasIdx: true, Params: []ParamMask{MASK_DIR}},
	"lld":   {Name: "lld", OpCode: OP_LLD, NbParams: 2, Cycles: 10, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_DIR | MASK_IND, MASK_REG}},
	"lldi":  {Name: "lldi", OpCode: OP_LLDI, NbParams: 3, Cycles: 50, HasPcode: true, HasIdx: true, Params: []ParamMask{MASK_REG | MASK_DIR | MASK_IND, MASK_REG | MASK_DIR, MASK_REG}},
	"lfork": {Name: "lfork", OpCode: OP_LFORK, NbParams: 1, Cycles: 1000, HasPcode: false, HasIdx: true, Params: []ParamMask{MASK_DIR}},
	"nop":   {Name: "nop", OpCode: OP_NOP, NbParams: 1, Cycles: 2, HasPcode: true, HasIdx: false, Params: []ParamMask{MASK_REG}},
}

var InstructionsByOp = map[OpCode]Instruction{}

func init() {
	for _, inst := range Instructions {
		InstructionsByOp[inst.OpCode] = inst
	}
}
