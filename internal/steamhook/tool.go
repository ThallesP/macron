package steamhook

const compatibilityToolVDF = `"compatibilitytools"
{
	"compat_tools"
	{
		"macron"
		{
			"install_path"	"."
			"display_name"	"Macron"
			"from_oslist"	"windows"
			"to_oslist"	"macos"
		}
	}
}
`

const toolManifestVDF = `"manifest"
{
	"version"	"2"
	"commandline"	"/macron %verb%"
}
`
