package main

import (
	"fmt"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/EpicStep/hatch/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	root := cli.NewRootCmd(version + " (" + commit + ")")
	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
