// revproxyhashry computes the hash of a password.
package main

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh/terminal"
)

func run() int {
	argsWoProg := os.Args[1:]

	logErr := log.New(os.Stderr, "", 0)

	if len(argsWoProg) > 1 {
		logErr.Printf("Expected at most one command-line argument, got: %#v\n", argsWoProg)
		return 1
	}

	var passwd []byte
	var err error

	switch {
	case len(argsWoProg) == 0:
		fmt.Printf("Please enter the password: ")
		passwd, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			logErr.Printf("failed to read the password: %s", err.Error())
			return 1
		}
		fmt.Println()

	case len(argsWoProg) == 1:
		passwd = []byte(argsWoProg[0])
	default:
		logErr.Printf("unhandled execution path")
		return 1
	}

	hsh, err := bcrypt.GenerateFromPassword(passwd, 14)
	if err != nil {
		logErr.Printf("failed to generate the hash: %s", err.Error())
		return 1
	}

	fmt.Println(string(hsh))

	return 0
}

func main() {
	os.Exit(run())
}
