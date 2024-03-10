package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/rpmpack"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type CliArgs struct {
	InputPath,
	OutputPath,
	PrivateKeyPath,
	PrivateKey string
}

func parseArgs() (CliArgs, error) {
	inputPath := flag.String("input-path", "/dev/stdin", "Input RPM file path (defaults to /dev/stdin)")
	outputPath := flag.String("output-path", "/dev/stdout", "Output RPM file path with changelog injected (defauls to /dev/stdout)")
	privateKeyPath := flag.String("private-key-path", "", "Private key path")
	flag.Parse()
	out := CliArgs{
		InputPath:      *inputPath,
		OutputPath:     *outputPath,
		PrivateKeyPath: *privateKeyPath,
	}
	if out.PrivateKeyPath == "" {
		return CliArgs{}, fmt.Errorf("-private-key-path is required")
	}
	return out, nil
}

func getSigningKey(key string) (*crypto.KeyRing, error) {
	privateKeyObj, err := crypto.NewKeyFromArmored(key);
	if err != nil {
		return nil, err
	}

	locked, err := privateKeyObj.IsLocked()
	if err != nil {
		return nil, err
	}

	if locked {
		return nil, fmt.Errorf("private key is locked");
	}

	return crypto.NewKeyRing(privateKeyObj);
}

func InternalMain(args CliArgs) (int){
	if (args.PrivateKey == "") {
		key, err := os.ReadFile(args.PrivateKeyPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to read private key: %w", err)
			return 2
		}
		args.PrivateKey = string(key)
	}

	signingKey, err := getSigningKey(args.PrivateKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read private key: %w", err)
		return 2
	}

	rpm, err := rpmpack.ReadRPMFile(args.InputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input RPM file: %w", err)
		return 2
	}

	rpm.SetPGPSigner(func(b []byte) ([]byte, error) {
		fmt.Fprintf(os.Stderr, "Signing RPM payload with a length of %d...\n", len(b))
		message := crypto.NewPlainMessage(b)
		out, err := signingKey.SignDetached(message)
		if err != nil {
			return nil, err
		}
		return out.Data, nil
	})

	w, err := os.OpenFile(args.OutputPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error opening output RPM file: %w", err)
		return 2
	}

	err = rpm.Write(w)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing output RPM file: %w", err)
		return 2
	}

	return 0
}

func main() {
	args, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(InternalMain(args))
}
