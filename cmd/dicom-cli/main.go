package main

import (
	"os"
)

func main() {
	os.Exit(Execute(os.Args[1:], ProductionRuntime()))
}
