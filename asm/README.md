# Corewar Assembler

Assembles `.s` assembly files into `.cor` binary format.

## Usage

```bash
go run asm/main.go <file.s>
```

This produces `<file.cor>` output.

## Build

```bash
cd asm
go build -o asm .
./asm <file.s>
```