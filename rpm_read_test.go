package rpmpack

import (
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestRealRpmReader(t *testing.T) {

	path, err := runfiles.Rlocation("some-centos9-rpm/file/downloaded")
	if err != nil {
		t.Errorf("Failed to get resource: %v", err)
		return
	}

	rpm, err := ReadRPMFile(path)
	if err != nil {
		t.Errorf("Failed to read rpm file: %v", err)
		return
	}

	if rpm == nil {
		t.Error("rpm is null")
	}

}
