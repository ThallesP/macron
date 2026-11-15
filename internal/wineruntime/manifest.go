package wineruntime

type Archive struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

type Manifest struct {
	Version     string  `json:"version"`
	Wine        Archive `json:"wine"`
	WineOverlay Archive `json:"wineOverlay"`
	DXMT        Archive `json:"dxmt"`
	DXVK        Archive `json:"dxvk"`
	SteamBridge Archive `json:"steamBridge"`
}

var Pinned = Manifest{
	Version: "wine-11.21-macron.6-dxmt-0.80-dxvk-1.10.3-20230507-bridge-proton-11.0-2e-macron.1",
	Wine: Archive{
		Name:      "wine-devel-11.21-osx64.tar.xz",
		URL:       "https://github.com/Gcenx/macOS_Wine_builds/releases/download/11.21/wine-devel-11.21-osx64.tar.xz",
		SHA256:    "9909b486657e10ab66c7279371ae1f990345b596b3b1b3a0cf21f28af24fa0ce",
		SizeBytes: 225612636,
	},
	WineOverlay: Archive{
		Name:      "wine-11.21-macron.6-overlay-osx64.tar.xz",
		URL:       "https://github.com/ThallesP/macron/releases/download/runtime-wine-11.21-macron.6/wine-11.21-macron.6-overlay-osx64.tar.xz",
		SHA256:    "3098ff955d2f48883d55f50fc0d9953c0120541704576ef1e847750a089b0827",
		SizeBytes: 11180032,
	},
	DXMT: Archive{
		Name:      "dxmt-v0.80-builtin.tar.gz",
		URL:       "https://github.com/3Shain/dxmt/releases/download/v0.80/dxmt-v0.80-builtin.tar.gz",
		SHA256:    "8f260e36b5739e68f3bad613381441385c4dc7b85b78ba8de653d5a6a264529d",
		SizeBytes: 18681669,
	},
	DXVK: Archive{
		Name:      "dxvk-macOS-async-v1.10.3-20230507.tar.gz",
		URL:       "https://github.com/Gcenx/DXVK-macOS/releases/download/v1.10.3-20230507/dxvk-macOS-async-v1.10.3-20230507.tar.gz",
		SHA256:    "f67d99d0a8eeedd7d406b283a3df9f939b5965acb00efcb33d0c6235c195a516",
		SizeBytes: 7077450,
	},
	SteamBridge: Archive{
		Name:      "macron-steam-bridge-proton-11.0-2e-macron.1-osx64.tar.xz",
		URL:       "https://github.com/ThallesP/macron/releases/download/steam-bridge-proton-11.0-2e-macron.1/macron-steam-bridge-proton-11.0-2e-macron.1-osx64.tar.xz",
		SHA256:    "9dbd3bc99b4bc247ca60ff4f6d6c35c29515f4938926f61fdba92ac72411983e",
		SizeBytes: 3285144,
	},
}
