package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/rpmpack"
)

type CliArgs struct {
	InputPath,
	OutputPath,
	ChangelogText string
	Index int
}

func parseArgs() (CliArgs, error) {
	inputPath := flag.String("input-path", "/dev/stdin", "Input RPM file path (defaults to /dev/stdin)")
	flag.Parse()
	out := CliArgs {
		InputPath: *inputPath,
	}
	return out, nil
}

func main() {
	args, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	rpm, err := rpmpack.ReadRPMFile(args.InputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input RPM file: %w", err)
		os.Exit(2)
	}

	changelog := rpm.GetChangelog()

	for i, entry := range(changelog) {
		fmt.Fprintf(os.Stdout, "Index: %d\nTime: %s\nAuthor: %s\nContent:\n%s\n\n", i, time.Unix(int64(entry.Time), 0), entry.Name, entry.Text)
	}
}
