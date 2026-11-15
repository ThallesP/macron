package steamhook

var KnownBuilds = []KnownBuild{
	{
		ID:      "1794536281",
		Variant: "macos-signed-2",
		Files: []KnownFile{
			{Path: "steamclient.dylib", SHA256: "55a9341f7e411f52bae049193a98071ae666b0fddb90ab3cae6aa527d9d84335", Package: "bins_client_osx.zip.c1d93228878a4ca2488f1da9ca24dde7197c1142"},
			{Path: "steam_osx", SHA256: "99cc08899ce366b9518c0a3782d1ae5954762894d2bf54cc2069c41df86162af", Package: "steam_osx_macos-signed-2.zip.76001f26b3188f51f20785f4922d2f12e2e6caec"},
		},
	},
	{
		ID:      "1794536281",
		Variant: "plain",
		Files: []KnownFile{
			{Path: "steamclient.dylib", SHA256: "55a9341f7e411f52bae049193a98071ae666b0fddb90ab3cae6aa527d9d84335", Package: "bins_client_osx.zip.c1d93228878a4ca2488f1da9ca24dde7197c1142"},
			{Path: "steam_osx", SHA256: "c15565c65c77a65322511e083caa1475d23dfbaa883a631494c4ef085ce6fdd1", Package: "steam_osx.zip.123e64db983cc79c9c7ccba3157882494939b1d9"},
		},
	},
}
