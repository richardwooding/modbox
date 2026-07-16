package modules

import _ "embed"

//go:embed drozerix_-_bit_loader.xm
var bitLoader []byte

//go:embed drozerix_-_crush.xm
var crush []byte

func init() {
	demos = []Demo{
		{
			Title:   "Bit Loader",
			Artist:  "Drozerix",
			License: "Public Domain",
			Source:  "https://modarchive.org/index.php?query=178415&request=view_by_moduleid",
			Data:    bitLoader,
		},
		{
			Title:   "Crush",
			Artist:  "Drozerix",
			License: "Public Domain",
			Source:  "https://modarchive.org/index.php?query=179581&request=view_by_moduleid",
			Data:    crush,
		},
	}
}
