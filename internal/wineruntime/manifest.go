package wineruntime

type Archive struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

type Manifest struct {
	Version string  `json:"version"`
	Wine    Archive `json:"wine"`
	DXMT    Archive `json:"dxmt"`
}

var Pinned = Manifest{
	Version: "wine-11.21-dxmt-0.80",
	Wine: Archive{
		Name:      "wine-devel-11.21-osx64.tar.xz",
		URL:       "https://github.com/Gcenx/macOS_Wine_builds/releases/download/11.21/wine-devel-11.21-osx64.tar.xz",
		SHA256:    "9909b486657e10ab66c7279371ae1f990345b596b3b1b3a0cf21f28af24fa0ce",
		SizeBytes: 225612636,
	},
	DXMT: Archive{
		Name:      "dxmt-v0.80-builtin.tar.gz",
		URL:       "https://github.com/3Shain/dxmt/releases/download/v0.80/dxmt-v0.80-builtin.tar.gz",
		SHA256:    "8f260e36b5739e68f3bad613381441385c4dc7b85b78ba8de653d5a6a264529d",
		SizeBytes: 18681669,
	},
}
