package main

import (
	"fmt"
	"os"

	"github.com/O6lvl4/ccp/profile"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	m, err := profile.NewManager()
	if err != nil {
		fatalf("%v", err)
	}

	switch os.Args[1] {
	case "init":
		requireArg(3, "ccp init <name>")
		if err := m.Init(os.Args[2]); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("created profile %q\n", os.Args[2])

	case "switch", "sw":
		if len(os.Args) < 3 {
			if err := m.SwitchDefault(); err != nil {
				fatalf("%v", err)
			}
			fmt.Println("switched to default (~/.claude)")
		} else {
			if err := m.Switch(os.Args[2]); err != nil {
				fatalf("%v", err)
			}
			fmt.Printf("switched to %q\n", os.Args[2])
		}

	case "list", "ls":
		names, err := m.List()
		if err != nil {
			fatalf("%v", err)
		}
		active, _ := m.Active()
		for _, name := range names {
			marker := "  "
			if name == active {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}

	case "status", "st":
		name := ""
		if len(os.Args) >= 3 {
			name = os.Args[2]
		} else {
			var ok bool
			name, ok = m.Active()
			if !ok {
				fmt.Println("no active profile (using default ~/.claude)")
				return
			}
		}
		statuses, err := m.Status(name)
		if err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("profile: %s\n\n", name)
		for _, s := range statuses {
			if s.Shared {
				fmt.Printf("  %-30s shared\n", s.Name)
			} else {
				fmt.Printf("  %-30s overridden\n", s.Name)
			}
		}

	case "override", "ov":
		requireArg(4, "ccp override <profile> <file>")
		if err := m.Override(os.Args[2], os.Args[3]); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("overridden %q in profile %q\n", os.Args[3], os.Args[2])

	case "share", "sh":
		requireArg(4, "ccp share <profile> <file>")
		if err := m.Share(os.Args[2], os.Args[3]); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("shared %q in profile %q\n", os.Args[3], os.Args[2])

	case "sync":
		requireArg(3, "ccp sync <profile>")
		added, err := m.Sync(os.Args[2])
		if err != nil {
			fatalf("%v", err)
		}
		if len(added) == 0 {
			fmt.Println("already up to date")
		} else {
			for _, a := range added {
				fmt.Printf("  + %s\n", a)
			}
		}

	case "delete", "rm":
		requireArg(3, "ccp delete <name>")
		if err := m.Delete(os.Args[2]); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("deleted profile %q\n", os.Args[2])

	case "env":
		fmt.Println(m.Env())

	case "shell-init":
		fmt.Print(shellInit())

	case "version", "--version", "-v":
		fmt.Printf("ccp %s\n", version)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func requireArg(n int, usage string) {
	if len(os.Args) < n {
		fatalf("usage: %s", usage)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ccp: "+format+"\n", args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Print(`ccp - Claude Code Profile switcher

Usage:
  ccp init <name>                Create a profile (symlinks from ~/.claude)
  ccp switch <name>              Switch to a profile
  ccp switch                     Switch back to default (~/.claude)
  ccp list                       List profiles
  ccp status [name]              Show profile file status (shared/overridden)
  ccp override <profile> <file>  Copy file from base, making it profile-specific
  ccp share <profile> <file>     Revert file to shared symlink
  ccp sync <profile>             Add symlinks for new files in ~/.claude
  ccp delete <name>              Delete a profile
  ccp env                        Print shell export for current profile
  ccp shell-init                 Print shell integration function

Aliases: switch=sw, list=ls, status=st, override=ov, share=sh, delete=rm

Shell integration (add to .zshrc / .bashrc):
  eval "$(ccp shell-init)"
`)
}

func shellInit() string {
	return `ccp() {
  case "$1" in
    switch|sw)
      command ccp "$@" && eval "$(command ccp env)"
      ;;
    *)
      command ccp "$@"
      ;;
  esac
}
`
}
