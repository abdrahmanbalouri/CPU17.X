# Corewar - ASM & VM

Projet Corewar: assembleur et machine virtuelle.

## Project Structure

```
coreware/
├── asm/           # Assembler (.s -> .cor)
├── vm/            # Virtual Machine (.cor executer)
├── players/       # Champion .s files
├── config.go      # Shared constants
└── vm_bin         # Pre-built VM binary
```

## Comment ca marche?

1. **Assembler (asm/)** - Lit les fichiers `.s` (assembly) et produit des fichiers `.cor` (binaire)
2. **VM (vm/)** - Execute les fichiers `.cor`, chaque processus execute les instructions de son champion

## Comment lancer

### 1. Assembler un champion

```bash
cd asm
go run main.go ../players/*.s
```

Cela cree `players/*.cor`

### 2. Lancer la VM

```bash
cd vm
go run main.go -d 1000 ../players/*.cor
```

Ou avec le binaire pre-built:

```bash
./vm_bin -d 1000 players/*.cor
```

## Options VM

- `-d N` : Affiche la memoire (dump) au cycle N

## Instructions supportees

| # | Instr | Description |
|---|-------|-------------|
| 1 | live   | Declare champion vivant |
| 2 | ld     | Load direct |
| 3 | st     | Store |
| 4 | add    | Addition |
| 5 | sub    | Soustraction |
| 6 | and    | ET logique |
| 7 | or     | OU logique |
| 8 | xor    | XOR logique |
| 9 | zjmp   | Jump if carry |
| 10 | ldi    | Load indirect indexe |
| 11 | sti    | Store indirect indexe |
| 12 | fork   | Fork process |
| 13 | lld    | Long load |
| 14 | lldi   | Long load indirect |
| 15 | lfork  | Long fork |
| 16 | nop    | No operation |