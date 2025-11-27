package main

import (
	"os"

	"kc/cmd"
)

func main() {
	os.Args = []string{
		"kc",
		"roles", "create",
		"--realm", "master",
		"--name", "example-role",
	}
	cmd.Execute()
}
