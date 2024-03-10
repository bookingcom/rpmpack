package main

import (
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/helper"
	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/google/rpmpack"
)

func TestRpmSigning(t *testing.T) {
	path, err := runfiles.Rlocation("some-centos9-rpm/file/downloaded")

	if err != nil {
		t.Errorf("Failed to get resource: %v", err)
		return
	}

	name := "Joe doe"
	email := "joe.doe@example.com"

	encKey, err := helper.GenerateKey(name, email, nil, "x25519", 0)
	if err != nil {
		t.Fatalf("Failed to create signing key: %+v", err)
	}

	args := CliArgs{
		InputPath:      path,
		OutputPath:     "output.rpm",
		PrivateKey: 	encKey,
	}

	ret := InternalMain(args)

	if ret != 0 {
		t.Fatalf("internalMain(args) != 0: %d", ret)
	}

	rpm, err := rpmpack.ReadRPMFile(args.OutputPath)

	if err != nil {
		t.Fatalf("failed to read output rpm: %+v", err)
	}

	signatures := rpm.GetSignatures()
	if signatures == nil {
		t.Fatalf("couldn't find any signature headers")
	}

	if signatures.CountHeaders() < 1 {
		t.Fatalf("expected to have at least some signature headers, count: %d", signatures.CountHeaders())
	}
}

