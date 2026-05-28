package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

type codeLine struct {
	Line int
	Text string
}

type argument struct {
	Raw   string
	Type  ParamType
	Value int32
	Label string
}

type ParsedInstruction struct {
	Name   string
	Args   []argument
	Line   int
	Offset int
}

func Assemble(inputPath, outputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	lines, err := readLines(file)
	if err != nil {
		return err
	}

	name, desc, codeLines, err := extractMeta(lines)
	if err != nil {
		return err
	}

	instructions, labels, err := parseProgram(codeLines)
	if err != nil {
		return err
	}

	code, err := encodeProgram(instructions, labels)
	if err != nil {
		return err
	}
	if len(code) > CHAMP_MAX_SIZE {
		return fmt.Errorf("program too large: %d bytes (max %d)", len(code), CHAMP_MAX_SIZE)
	}

	return writeBinary(outputPath, name, desc, code)
}

func readLines(file *os.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return lines, nil
}

func extractMeta(lines []string) (string, string, []codeLine, error) {
	var name, desc string
	var code []codeLine

	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, ".name"):
			value, err := quotedDirective(line, ".name", lineNo)
			if err != nil {
				return "", "", nil, err
			}
			if len(value) > PROG_NAME_LENGTH {
				return "", "", nil, &ParseError{Line: lineNo, Msg: "name is too long"}
			}
			name = value
		case strings.HasPrefix(line, ".description"):
			value, err := quotedDirective(line, ".description", lineNo)
			if err != nil {
				return "", "", nil, err
			}
			if len(value) > COMMENT_LENGTH {
				return "", "", nil, &ParseError{Line: lineNo, Msg: "description is too long"}
			}
			desc = value
		default:
			code = append(code, codeLine{Line: lineNo, Text: line})
		}
	}

	if name == "" {
		return "", "", nil, &ParseError{Line: 1, Msg: "no .name directive found"}
	}
	if desc == "" {
		return "", "", nil, &ParseError{Line: 1, Msg: "no .description directive found"}
	}
	return name, desc, code, nil
}

func quotedDirective(line, directive string, lineNo int) (string, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, directive))
	if !strings.HasPrefix(rest, "\"") {
		return "", &ParseError{Line: lineNo, Msg: "missing opening quote for " + directive}
	}
	rest = rest[1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return "", &ParseError{Line: lineNo, Msg: "missing closing quote for " + directive}
	}
	if strings.TrimSpace(rest[end+1:]) != "" {
		return "", &ParseError{Line: lineNo, Msg: "unexpected text after " + directive}
	}
	return rest[:end], nil
}

func stripComment(line string) string {
	inQuote := false
	for i, r := range line {
		if r == '"' {
			inQuote = !inQuote
		}
		if !inQuote && (r == '#' || r == ';') {
			return line[:i]
		}
	}
	return line
}

func parseProgram(lines []codeLine) ([]ParsedInstruction, map[string]int, error) {
	var instructions []ParsedInstruction
	labels := make(map[string]int)
	offset := 0

	for _, src := range lines {
		rest := strings.TrimSpace(src.Text)
		for {
			label, tail, ok, err := takeLabel(rest, src.Line)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				break
			}
			if _, exists := labels[label]; exists {
				return nil, nil, &ParseError{Line: src.Line, Msg: "repeated label: " + label}
			}
			labels[label] = offset
			rest = strings.TrimSpace(tail)
			if rest == "" {
				break
			}
		}
		if rest == "" {
			continue
		}

		inst, err := parseInstruction(rest, src.Line, offset)
		if err != nil {
			return nil, nil, err
		}
		instructions = append(instructions, inst)
		offset += instructionSize(inst)
	}

	return instructions, labels, nil
}

func takeLabel(line string, lineNo int) (string, string, bool, error) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", line, false, nil
	}
	firstSpace := strings.IndexFunc(line, unicode.IsSpace)
	if firstSpace >= 0 && firstSpace < colon {
		return "", line, false, nil
	}
	label := strings.TrimSpace(line[:colon])
	if label == "" {
		return "", "", false, &ParseError{Line: lineNo, Msg: "empty label"}
	}
	if !validLabel(label) {
		return "", "", false, &ParseError{Line: lineNo, Msg: "invalid label: " + label}
	}
	return label, line[colon+1:], true, nil
}

func validLabel(label string) bool {
	for _, r := range label {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

func parseInstruction(line string, lineNo, offset int) (ParsedInstruction, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ParsedInstruction{}, &ParseError{Line: lineNo, Msg: "empty instruction"}
	}

	name := fields[0]
	instDef, ok := Instructions[name]
	if !ok {
		return ParsedInstruction{}, &ParseError{Line: lineNo, Msg: "unknown instruction: " + name}
	}

	paramText := strings.TrimSpace(line[len(name):])
	parts := splitParams(paramText)
	if len(parts) != instDef.NbParams {
		return ParsedInstruction{}, &ParseError{Line: lineNo, Msg: fmt.Sprintf("%s expects %d parameter(s), got %d", name, instDef.NbParams, len(parts))}
	}

	args := make([]argument, 0, len(parts))
	for i, part := range parts {
		arg, err := parseArgument(part, lineNo)
		if err != nil {
			return ParsedInstruction{}, err
		}
		if instDef.Params[i]&maskFor(arg.Type) == 0 {
			return ParsedInstruction{}, &ParseError{Line: lineNo, Msg: fmt.Sprintf("invalid parameter %d for %s: %s", i+1, name, part)}
		}
		args = append(args, arg)
	}

	return ParsedInstruction{Name: name, Args: args, Line: lineNo, Offset: offset}, nil
}

func splitParams(paramText string) []string {
	if strings.TrimSpace(paramText) == "" {
		return nil
	}
	raw := strings.Split(paramText, ",")
	params := make([]string, 0, len(raw))
	for _, part := range raw {
		params = append(params, strings.TrimSpace(part))
	}
	return params
}

func parseArgument(raw string, lineNo int) (argument, error) {
	if raw == "" {
		return argument{}, &ParseError{Line: lineNo, Msg: "empty parameter"}
	}

	if strings.HasPrefix(raw, "r") || strings.HasPrefix(raw, "R") {
		n, err := strconv.Atoi(raw[1:])
		if err != nil || n < 1 || n > REG_NUMBER {
			return argument{}, &ParseError{Line: lineNo, Msg: "invalid register: " + raw}
		}
		return argument{Raw: raw, Type: PARAM_REG, Value: int32(n)}, nil
	}

	if strings.HasPrefix(raw, "%:") {
		label := raw[2:]
		if !validLabel(label) {
			return argument{}, &ParseError{Line: lineNo, Msg: "invalid label reference: " + raw}
		}
		return argument{Raw: raw, Type: PARAM_DIR, Label: label}, nil
	}

	if strings.HasPrefix(raw, "%") {
		n, err := parseNumber(raw[1:], lineNo)
		if err != nil {
			return argument{}, err
		}
		return argument{Raw: raw, Type: PARAM_DIR, Value: n}, nil
	}

	if strings.HasPrefix(raw, ":") {
		label := raw[1:]
		if !validLabel(label) {
			return argument{}, &ParseError{Line: lineNo, Msg: "invalid label reference: " + raw}
		}
		return argument{Raw: raw, Type: PARAM_IND, Label: label}, nil
	}

	n, err := parseNumber(raw, lineNo)
	if err != nil {
		return argument{}, err
	}
	return argument{Raw: raw, Type: PARAM_IND, Value: n}, nil
}

func parseNumber(s string, lineNo int) (int32, error) {
	if strings.TrimSpace(s) == "" {
		return 0, &ParseError{Line: lineNo, Msg: "empty number"}
	}
	n, err := strconv.ParseInt(s, 0, 32)
	if err != nil {
		return 0, &ParseError{Line: lineNo, Msg: "invalid number: " + s}
	}
	return int32(n), nil
}

func instructionSize(inst ParsedInstruction) int {
	def := Instructions[inst.Name]
	size := OPCODE_SIZE
	if def.HasPcode {
		size += PCODE_SIZE
	}
	for _, arg := range inst.Args {
		size += argumentSize(arg.Type, def.HasIdx)
	}
	return size
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

func encodeProgram(instructions []ParsedInstruction, labels map[string]int) ([]byte, error) {
	var code []byte
	for _, inst := range instructions {
		bin, err := encodeInstruction(inst, labels)
		if err != nil {
			return nil, err
		}
		code = append(code, bin...)
	}
	return code, nil
}

func encodeInstruction(inst ParsedInstruction, labels map[string]int) ([]byte, error) {
	def := Instructions[inst.Name]
	out := []byte{byte(def.OpCode)}
	if def.HasPcode {
		out = append(out, pcode(inst.Args))
	}

	for _, arg := range inst.Args {
		value := arg.Value
		if arg.Label != "" {
			labelOffset, ok := labels[arg.Label]
			if !ok {
				return nil, &ParseError{Line: inst.Line, Msg: "unknown label: " + arg.Label}
			}
			value = int32(labelOffset - inst.Offset)
		}

		switch arg.Type {
		case PARAM_REG:
			out = append(out, byte(value))
		case PARAM_DIR:
			if def.HasIdx {
				out = appendI16(out, int16(value))
			} else {
				out = appendI32(out, value)
			}
		case PARAM_IND:
			out = appendI16(out, int16(value))
		}
	}

	return out, nil
}

func pcode(args []argument) byte {
	var code byte
	for i, arg := range args {
		code |= byte(arg.Type) << (6 - i*2)
	}
	return code
}

func appendI16(out []byte, v int16) []byte {
	u := uint16(v)
	return append(out, byte(u>>8), byte(u))
}

func appendI32(out []byte, v int32) []byte {
	u := uint32(v)
	return append(out, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

func writeBinary(path, name, desc string, code []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer f.Close()

	if err := binary.Write(f, binary.BigEndian, MAGIC); err != nil {
		return err
	}
	if err := writePadded(f, []byte(name), PROG_NAME_LENGTH); err != nil {
		return err
	}
	if err := binary.Write(f, binary.BigEndian, uint32(len(code))); err != nil {
		return err
	}
	if err := writePadded(f, []byte(desc), COMMENT_LENGTH); err != nil {
		return err
	}
	_, err = f.Write(code)
	return err
}

func writePadded(w io.Writer, data []byte, size int) error {
	if len(data) > size {
		data = data[:size]
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if len(data) < size {
		_, err := w.Write(make([]byte, size-len(data)))
		return err
	}
	return nil
}
