package types

type File struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type LibraryDescription struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Sketch struct {
	Name     string          `json:"name"`
	Files    []File          `json:"files"`
	Ino      File            `json:"ino"`
	Metadata *SketchMetadata `json:"metadata"`
}

type SketchMetadata struct {
	IncludedLibs []LibraryDescription `json:"includedLibs"`
}
