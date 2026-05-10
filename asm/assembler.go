package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type TokenType int

const (
	TOKEN_LABEL TokenType = iota
	TOKEN_INSTRUCTION
	TOKEN_REGISTER
	TOKEN_DIRECT
	TOKEN_INDIRECT
	TOKEN_COMMA
	TOKEN_NEWLINE
	TOKEN_DOT_NAME
	TOKEN_DOT_DESC
	TOKEN_EOF
)

type Token struct {
	Type  TokenType
	Value string
	Line  int
}

type ParsedInstruction struct {
	Label    string
	Name     string
	Params   []string
	Line     int
}

type Label struct {
	Name string
	Pos  int
}

type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func Assemble(inputPath, outputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open file: %v", err)
	}
	defer file.Close()

	lines, err := readLines(file)
	if err != nil {
		return err
	}

	// First pass: find .name and .description
	name, desc, codeLines, err := extractMeta(lines)
	if err != nil {
		return err
	}

	// Second pass: find labels and their positions
	labels := findLabels(codeLines)

	// Third pass: parse instructions and resolve labels
	instructions, err := parseInstructions(codeLines, labels)
	if err != nil {
		return err
	}

	// Generate binary
	return generateBinary(outputPath, name, desc, instructions)
}

func readLines(file *os.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	lines = append(lines, "")
	return lines, nil
}

func extractMeta(lines []string) (name, desc string, codeLines []string, err error) {
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ".name") {
			parts := strings.SplitN(line, "\"", 2)
			if len(parts) < 2 {
				return "", "", nil, &ParseError{Line: i + 1, Msg: "invalid .name syntax"}
			}
			parts = strings.SplitN(parts[1], "\"", 2)
			name = parts[0]
			continue
		}
		if strings.HasPrefix(line, ".description") {
			parts := strings.SplitN(line, "\"", 2)
			if len(parts) < 2 {
				return "", "", nil, &ParseError{Line: i + 1, Msg: "invalid .description syntax"}
			}
			parts = strings.SplitN(parts[1], "\"", 2)
			desc = parts[0]
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ";") {
			codeLines = append(codeLines, line)
		}
	}
	if name == "" {
		return "", "", nil, &ParseError{Line: 1, Msg: "no .name directive found"}
	}
	return name, desc, codeLines, nil
}

func findLabels(lines []string) map[string]int {
	labels := make(map[string]int)
	pos := 0

	for _, line := range lines {
		parts := strings.Fields(line)
		for _, part := range parts {
			if strings.HasSuffix(part, ":") {
				label := strings.TrimSuffix(part, ":")
				labels[label] = pos
			}
		}
		if len(parts) > 0 && parts[0] != "" {
			_, ok := Instructions[parts[0]]
			if ok {
				pos += 1 + opcodeSize(parts[0], len(parts)-1)
			}
		}
	}
	return labels
}

func parseInstructions(lines []string, labels map[string]int) ([]ParsedInstruction, error) {
	var instructions []ParsedInstruction

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		inst, err := parseLine(line, labels, lineNum+1)
		if err != nil {
			return nil, err
		}
		if inst != nil {
			instructions = append(instructions, *inst)
		}
	}
	return instructions, nil
}

func parseLine(line string, labels map[string]int, lineNum int) (*ParsedInstruction, error) {
	var inst ParsedInstruction
	inst.Line = lineNum

	// Handle labels - they can be at start or end of instruction
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, nil
	}

	// Check if first part is a label (ends with :)
	if strings.HasSuffix(parts[0], ":") {
		inst.Label = strings.TrimSuffix(parts[0], ":")
		parts = parts[1:]
		if len(parts) == 0 {
			return nil, nil // just a label declaration, no instruction
		}
	}

	// Now parts[0] should be the instruction
	inst.Name = parts[0]
	if _, ok := Instructions[inst.Name]; !ok {
		return nil, &ParseError{Line: lineNum, Msg: fmt.Sprintf("unknown instruction: %s", inst.Name)}
	}

	if len(parts) > 1 {
		paramStr := strings.Join(parts[1:], "")
		inst.Params = strings.Split(paramStr, ",")
		for i, p := range inst.Params {
			inst.Params[i] = strings.TrimSpace(p)
		}
	}

	return &inst, nil
}

func opcodeSize(name string, paramCount int) int {
	inst, ok := Instructions[name]
	if !ok {
		return 0
	}

	size := 0
	if inst.HasPcode {
		size += 1
	}

	for i := 0; i < inst.NbParams; i++ {
		paramType := inst.Params[i]
		if paramType == PARAM_REG {
			size += 1
		} else if inst.HasIdx {
			size += IND_SIZE
		} else {
			size += DIR_SIZE
		}
	}
	return size
}

func generateBinary(path, name, desc string, instructions []ParsedInstruction) error {
	// First pass: compute label positions
	labelPositions := make(map[string]int)
	pos := 0
	for _, inst := range instructions {
		if inst.Label != "" {
			labelPositions[inst.Label] = pos
		}
		bin, _ := encodeInstruction(inst, labelPositions)
		pos += len(bin)
	}

	// Second pass: encode with resolved labels
	code := make([]byte, 0)
	for _, inst := range instructions {
		bin, err := encodeInstruction(inst, labelPositions)
		if err != nil {
			return err
		}
		code = append(code, bin...)
	}

	if len(code) > CHAMP_MAX_SIZE {
		return fmt.Errorf("program too large: %d bytes (max %d)", len(code), CHAMP_MAX_SIZE)
	}

	// Create file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create output file: %v", err)
	}
	defer f.Close()

	// Write magic (big-endian)
	binary.Write(f, binary.BigEndian, MAGIC)

	// Write name (null-padded to 128 bytes)
	nameBytes := []byte(name)
	if len(nameBytes) > PROG_NAME_LENGTH {
		nameBytes = nameBytes[:PROG_NAME_LENGTH]
	}
	f.Write(nameBytes)
	if len(nameBytes) < PROG_NAME_LENGTH {
		f.Write(make([]byte, PROG_NAME_LENGTH-len(nameBytes)))
	}

	// Write program size (big-endian)
	binary.Write(f, binary.BigEndian, uint32(len(code)))

	// Write description (null-padded to 2048 bytes)
	descBytes := []byte(desc)
	if len(descBytes) > COMMENT_LENGTH {
		descBytes = descBytes[:COMMENT_LENGTH]
	}
	f.Write(descBytes)
	if len(descBytes) < COMMENT_LENGTH {
		f.Write(make([]byte, COMMENT_LENGTH-len(descBytes)))
	}

	// Write code
	f.Write(code)

	return nil
}

func encodeInstruction(inst ParsedInstruction, labelPositions map[string]int) ([]byte, error) {
	instDef, ok := Instructions[inst.Name]
	if !ok {
		return nil, fmt.Errorf("unknown instruction: %s", inst.Name)
	}

	result := make([]byte, 0)

	// Write opcode
	result = append(result, byte(instDef.OpCode))

	// Write pcode if needed
	if instDef.HasPcode {
		pcode := calculatePcode(inst.Name, inst.Params)
		result = append(result, pcode)
	}

	// Calculate position after this instruction for label resolution
	currentPos := len(result)

	// Write parameters
	for i, param := range inst.Params {
		if i >= len(instDef.Params) {
			break
		}

		paramType := instDef.Params[i]
		val, err := parseParameter(param, paramType, instDef.HasIdx, inst.Line, labelPositions, currentPos)
		if err != nil {
			return nil, err
		}

		if paramType == PARAM_REG {
			result = append(result, byte(val))
			currentPos++
		} else if instDef.HasIdx {
			// 2 bytes
			result = append(result, byte((val>>8)&0xFF), byte(val&0xFF))
			currentPos += 2
		} else {
			// 4 bytes
			result = append(result,
				byte((val>>24)&0xFF),
				byte((val>>16)&0xFF),
				byte((val>>8)&0xFF),
				byte(val&0xFF))
			currentPos += 4
		}
	}

	return result, nil
}

func calculatePcode(instName string, params []string) byte {
	inst, ok := Instructions[instName]
	if !ok {
		return 0
	}

	var pcode byte
	shift := 6

	for i := 0; i < len(params) && i < len(inst.Params); i++ {
		var typeBits byte
		if strings.HasPrefix(params[i], "r") {
			typeBits = 0x01 // register
		} else if strings.HasPrefix(params[i], "%") {
			typeBits = 0x02 // direct
		} else {
			typeBits = 0x03 // indirect
		}
		pcode |= typeBits << shift
		shift -= 2
	}

	return pcode
}

func parseParameter(param string, expectedType ParamType, hasIdx bool, line int, labelPositions map[string]int, currentPos int) (int, error) {
	param = strings.TrimSpace(param)

	// Register
	if strings.HasPrefix(param, "r") || strings.HasPrefix(param, "R") {
		regNum, err := strconv.Atoi(strings.TrimPrefix(param, "r"))
		if err != nil {
			regNum, _ = strconv.Atoi(strings.TrimPrefix(param, "R"))
		}
		if regNum < 1 || regNum > REG_NUMBER {
			return 0, &ParseError{Line: line, Msg: fmt.Sprintf("invalid register: %s", param)}
		}
		return regNum, nil
	}

	// Direct with label (e.g., %:label)
	if strings.HasPrefix(param, "%:") {
		label := strings.TrimPrefix(param, "%:")
		labelPos, ok := labelPositions[label]
		if !ok {
			return 0, &ParseError{Line: line, Msg: fmt.Sprintf("unknown label: %s", label)}
		}
		val := labelPos - currentPos
		if hasIdx {
			val = val % IDX_MOD
		}
		return val, nil
	}

	// Direct with % prefix
	if strings.HasPrefix(param, "%") {
		val, err := parseNumber(strings.TrimPrefix(param, "%"), line)
		if err != nil {
			return 0, err
		}
		if hasIdx {
			val = val % IDX_MOD
		}
		return val, nil
	}

	// Indirect with label (e.g., :label)
	if strings.HasPrefix(param, ":") {
		label := strings.TrimPrefix(param, ":")
		labelPos, ok := labelPositions[label]
		if !ok {
			return 0, &ParseError{Line: line, Msg: fmt.Sprintf("unknown label: %s", label)}
		}
		val := labelPos - currentPos
		if hasIdx {
			val = val % IDX_MOD
		}
		return val, nil
	}

	// Indirect - could be a number
	if strings.ContainsAny(param, "-+") || isNumeric(param) {
		val, err := parseNumber(param, line)
		if err != nil {
			return 0, err
		}
		if hasIdx {
			val = val % IDX_MOD
		}
		return val, nil
	}

	// Must be a label - this should be resolved in second pass
	return 0, &ParseError{Line: line, Msg: fmt.Sprintf("invalid parameter: %s", param)}
}

func isNumeric(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func parseNumber(s string, line int) (int, error) {
	s = strings.TrimSpace(s)

	// Handle negative hex
	if strings.HasPrefix(s, "-0x") || strings.HasPrefix(s, "-0X") {
		v, err := strconv.ParseInt(s, 0, 32)
		if err != nil {
			return 0, &ParseError{Line: line, Msg: fmt.Sprintf("invalid number: %s", s)}
		}
		return int(v), nil
	}

	// Handle positive hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseInt(s, 0, 32)
		if err != nil {
			return 0, &ParseError{Line: line, Msg: fmt.Sprintf("invalid number: %s", s)}
		}
		return int(v), nil
	}

	// Regular number
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, &ParseError{Line: line, Msg: fmt.Sprintf("invalid number: %s", s)}
	}
	return int(v), nil
}