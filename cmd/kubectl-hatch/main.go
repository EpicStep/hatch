package main

import (
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/EpicStep/hatch/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	root := cli.NewRootCmd(version+" ("+commit+")", streams)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(streams.ErrOut, err)
		os.Exit(1)
	}
}
