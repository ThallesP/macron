package fixes

import "github.com/ThallesP/macron/internal/wineruntime"

type InstallStep struct {
	Archive wineruntime.Archive
	Args    []string
}

type Installer struct {
	Steps         []InstallStep
	GoodExitCodes []int
}

var Installers = map[string]Installer{
	"vcrun2022": {
		Steps: []InstallStep{
			{
				Archive: wineruntime.Archive{
					Name:      "VC_redist.x64.exe",
					URL:       "https://download.visualstudio.microsoft.com/download/pr/e9604e21-4b79-4096-a4b3-1b63d4b55b4d/CC0FF0EB1DC3F5188AE6300FAEF32BF5BEEBA4BDD6E8E445A9184072096B713B/VC_redist.x64.exe",
					SHA256:    "cc0ff0eb1dc3f5188ae6300faef32bf5beeba4bdd6e8e445a9184072096b713b",
					SizeBytes: 25635768,
				},
				Args: []string{"/install", "/quiet", "/norestart"},
			},
			{
				Archive: wineruntime.Archive{
					Name:      "VC_redist.x86.exe",
					URL:       "https://download.visualstudio.microsoft.com/download/pr/e9604e21-4b79-4096-a4b3-1b63d4b55b4d/0C09F2611660441084CE0DF425C51C11E147E6447963C3690F97E0B25C55ED64/VC_redist.x86.exe",
					SHA256:    "0c09f2611660441084ce0df425c51c11e147e6447963c3690f97e0b25c55ed64",
					SizeBytes: 13953392,
				},
				Args: []string{"/install", "/quiet", "/norestart"},
			},
		},
		GoodExitCodes: []int{0, 1638, 3010},
	},
}
