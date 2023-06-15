# GoHeif - A go gettable decoder/converter for HEIC based on libde265

Intel and ARM supported

## Install

```go get github.com/klippa-app/goheif```

- Code Sample

First make a worker package/binary in the libde265_plugin directory.

```
package main

import "github.com/klippa-app/goheif/libde265/plugin"

func main() {
	plugin.StartPlugin()
}
```

Then use the worker file/binary in your program.
If you want to make it run through go, use the example below.

You can also go build `libde265_plugin/main.go` and then reference it in BinPath, this is the advices run method for deployments.

```
package main

func init() {
    err := goheif.Init(Config{Lib265Config: libde265.Config{
		Command: libde265.Command{
			BinPath: "go",
			Args:    []string{"run", "libde265_plugin/main.go"},
		},
	}})
}

func main() {
	flag.Parse()
	...
  
	fin, fout := flag.Arg(0), flag.Arg(1)
	fi, err := os.Open(fin)
	if err != nil {
		log.Fatal(err)
	}
	defer fi.Close()

	exif, err := goheif.ExtractExif(fi)
	if err != nil {
		log.Printf("Warning: no EXIF from %s: %v\n", fin, err)
	}

	img, err := goheif.DecodeImage(fi)
	if err != nil {
		log.Fatalf("Failed to parse %s: %v\n", fin, err)
	}

	fo, err := os.OpenFile(fout, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Failed to create output file %s: %v\n", fout, err)
	}
	defer fo.Close()

	w, _ := newWriterExif(fo, exif)
	err = jpeg.Encode(w, img, nil)
	if err != nil {
		log.Fatalf("Failed to encode %s: %v\n", fout, err)
	}

	log.Printf("Convert %s to %s successfully\n", fin, fout)
}
```

## What is done

- Changes make to @bradfitz's (https://github.com/bradfitz) golang heif parser
  - Some minor bugfixes
  - A few new box parsers, noteably 'iref' and 'hvcC'

- Includes libde265 using pkg-config and a simple golang binding

- Processes the images in a subprocess to prevent crashing the main application on segfaults

- A Utility `heic2jpg` to illustrate the usage.

## License

- heif and libde265 are in their own licenses

- goheif.go, libde265 golang binding and the `heic2jpg` utility are in MIT license

## Credits
- heif parser by @bradfitz (https://github.com/go4org/go4/tree/master/media/heif)
- libde265 (https://github.com/strukturag/libde265)
- implementation learnt from libheif (https://github.com/strukturag/libheif)



