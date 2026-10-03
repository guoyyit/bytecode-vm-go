package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
	"github.com/guoyyit/bytecode-vm-go/vm"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "info":
		infoCmd(os.Args[2:])
	case "version":
		fmt.Println("bvm version 1.0.0")
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage: bvm <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  run <file> [--entry func] [--args 1,2,3] [--max-steps N] [--max-memory N] [--max-depth N]")
	fmt.Println("  info <file>")
	fmt.Println("  version")
}

func runCmd(args []string) {
	entry := "main"
	argStr := ""
	maxSteps := vm.DefaultConfig().MaxSteps
	maxMemory := vm.DefaultConfig().MaxMemoryBytes
	maxDepth := vm.DefaultConfig().MaxCallDepth
	maxStack := vm.DefaultConfig().MaxStackDepth
	var file string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--entry":
			i++
			entry = args[i]
		case "--args":
			i++
			argStr = args[i]
		case "--max-steps":
			i++
			v, err := strconv.ParseUint(args[i], 10, 64)
			if err != nil {
				fmt.Printf("error parsing --max-steps: %v\n", err)
				os.Exit(1)
			}
			maxSteps = v
		case "--max-memory":
			i++
			v, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				fmt.Printf("error parsing --max-memory: %v\n", err)
				os.Exit(1)
			}
			maxMemory = v
		case "--max-depth":
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Printf("error parsing --max-depth: %v\n", err)
				os.Exit(1)
			}
			maxDepth = v
		case "--max-stack":
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Printf("error parsing --max-stack: %v\n", err)
				os.Exit(1)
			}
			maxStack = v
		default:
			if strings.HasPrefix(a, "--") {
				fmt.Printf("unknown flag: %s\n", a)
				os.Exit(1)
			}
			file = a
		}
	}

	if file == "" {
		fmt.Println("error: missing bytecode file")
		os.Exit(1)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("error reading file: %v\n", err)
		os.Exit(1)
	}

	mod, err := bytecode.Load(data)
	if err != nil {
		fmt.Printf("error loading bytecode: %v\n", err)
		os.Exit(1)
	}

	var vmArgs []vm.Value
	if argStr != "" {
		for _, s := range strings.Split(argStr, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				fmt.Printf("error parsing argument %q: %v\n", s, err)
				os.Exit(1)
			}
			vmArgs = append(vmArgs, vm.IntValue(n))
		}
	}

	cfg := vm.Config{
		MaxSteps:       maxSteps,
		MaxMemoryBytes: maxMemory,
		MaxCallDepth:   maxDepth,
		MaxStackDepth:  maxStack,
	}

	result, err := vm.Execute(mod, cfg, entry, vmArgs)
	if err != nil {
		fmt.Printf("execution error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("result: %s\n", result.Value)
	fmt.Printf("steps: %d\n", result.Steps)
}

func infoCmd(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Println("error: missing bytecode file")
		os.Exit(1)
	}

	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Printf("error reading file: %v\n", err)
		os.Exit(1)
	}

	mod, err := bytecode.Load(data)
	if err != nil {
		fmt.Printf("error loading bytecode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("bytecode version: %s\n", mod.VersionString())
	fmt.Printf("format version: %d\n", mod.FormatVersion)
	fmt.Printf("flags: 0x%04x\n", mod.Flags)
	fmt.Printf("constants: %d\n", mod.ConstantCount())
	for i, c := range mod.Constants {
		fmt.Printf("  [%d] %v\n", i, c)
	}
	fmt.Printf("functions: %d\n", mod.FunctionCount())
	for _, f := range mod.Functions {
		fmt.Printf("  %s (params=%d, locals=%d, code=%d bytes)\n", f.Name, f.NumParams, f.NumLocals, len(f.Code))
	}
}
