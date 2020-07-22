package types

type Package struct {
	GoPackagePath string
	URL           string
	Rev           string
	Sha256        string
	SumVersion    string
	RelPath       string
	VendorPath    string
}
