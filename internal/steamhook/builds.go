package steamhook

type KnownBuild struct {
	ID    string
	Files []KnownFile
}

type KnownFile struct {
	Path   string
	SHA256 string
}

var KnownBuilds = []KnownBuild{
	{
		ID: "1794536281",
		Files: []KnownFile{
			{Path: "steamclient.dylib", SHA256: "55a9341f7e411f52bae049193a98071ae666b0fddb90ab3cae6aa527d9d84335"},
			{Path: "steam_osx", SHA256: "99cc08899ce366b9518c0a3782d1ae5954762894d2bf54cc2069c41df86162af"},
		},
	},
}
