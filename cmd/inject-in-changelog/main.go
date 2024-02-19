package main

import (
	"flag"
	"fmt"
	"os"

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
	outputPath := flag.String("output-path", "/dev/stdout", "Output RPM file path with changelog injected (defauls to /dev/stdout)")
	changelogText := flag.String("changelog-text", "", "Changelog text to inject")
	index := flag.Int("index", 0, "Index to modify changelog entry (defaults to the first one)")
	flag.Parse()
	out := CliArgs {
		InputPath: *inputPath,
		OutputPath: *outputPath,
		ChangelogText: *changelogText,
		Index: *index,
	}
	if out.ChangelogText == "" {
		return CliArgs{}, fmt.Errorf("-changelog-text is required")
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

	if len(changelog) == 0 {
		fmt.Fprintln(os.Stderr, "There's no changelog in the rpm file")
		os.Exit(2)
	}

	if args.Index < 0 {
		args.Index = len(changelog) + args.Index
	}

	if args.Index < 0 {
		fmt.Fprintf(os.Stderr, "Effective index is not possitive %d %d\n", args.Index, len(changelog))
		os.Exit(2)
	}

	if len(changelog) < args.Index {
		fmt.Fprintf(os.Stderr, "Effective index is out of bounds %d %d\n", args.Index, len(changelog))
		os.Exit(2)
	}

	entry := changelog[args.Index]
	entry.Text = fmt.Sprintf("%s\n%s", entry.Text, args.ChangelogText)

	w, err := os.OpenFile(args.OutputPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening output RPM file: %w", err)
		os.Exit(2)
	}

	err = rpm.Write(w)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing output RPM file: %w", err)
		os.Exit(2)
	}

	fmt.Fprintln(os.Stderr, "Succesfully injected changelog text")
}
