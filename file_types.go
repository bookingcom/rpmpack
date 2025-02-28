package rpmpack

// FileType is the type of a file inside a RPM package.
type FileType int32

// https://refspecs.linuxbase.org/LSB_3.1.1/LSB-Core-generic/LSB-Core-generic/pkgformat.html#AEN27560
// The RPMFile.Type tag value shall identify various characteristics of the file in the payload that it describes.
// It shall be an INT32 value consisting of either the value GenericFile (0) or
// the bitwise inclusive or of one or more of the following values. Some of these combinations may make no sense
const (
	// GenericFile is just a basic file in an RPM
	GenericFile FileType = 1 << iota >> 1
	// ConfigFile is a configuration file, and an existing file should be saved during a
	// package upgrade operation and not removed during a package removal operation.
	ConfigFile
	// DocFile is a file that contains documentation.
	DocFile
	// DoNotUseFile is reserved for future use; conforming packages may not use this flag.
	DoNotUseFile
	// MissingOkFile need not exist on the installed system.
	MissingOkFile
	// NoReplaceFile similar to the ConfigFile, this flag indicates that during an upgrade operation
	// the original file on the system should not be altered.
	NoReplaceFile
	// SpecFile is the package specification file.
	SpecFile
	// GhostFile is not actually included in the payload, but should still be considered as a part of the package.
	// For example, a log file generated by the application at run time.
	GhostFile
	// LicenceFile contains the license conditions.
	LicenceFile
	// ReadmeFile contains high level notes about the package.
	ReadmeFile
	NonUsed0
	NonUsed1
	PubKey
	Artifact
)

// RPMFile contains a particular file's entry and data.
type RPMFile struct {
	Name  string
	Body  []byte
	Mode  uint
	Owner string
	Group string
	MTime uint32
	Type  FileType
}
