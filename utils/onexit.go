package utils

import (
	"os"
)

var funcs = make([]func(), 0)

// OnExit registers a exit function.
func OnExit(f func()) {
	funcs = append(funcs, f)
}

// CallOnExit calls the exit functions.
func CallOnExit() {
	for _, f := range funcs {
		f()
	}
}

// Exit exits the process with the code, but calling the exit functions
// before exiting.
func Exit(code int) {
	CallOnExit()
	os.Exit(code)
}