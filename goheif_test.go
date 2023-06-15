package goheif

import (
	"bytes"
	"image"
	"io"
	"io/ioutil"
	"testing"

	"github.com/klippa-app/goheif/libde265"
)

func initLib() error {
	err := Init(Config{Lib265Config: libde265.Config{
		Command: libde265.Command{
			BinPath: "go",
			Args:    []string{"run", "libde265/worker_example/main.go"},
		},
	}})
	if err != nil {
		return err
	}
	return nil
}

func TestFormatRegistered(t *testing.T) {
	err := initLib()
	if err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadFile("testdata/camel.heic")
	if err != nil {
		t.Fatal(err)
	}

	img, dec, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("unable to decode heic image: %s", err)
	}

	if got, want := dec, "heic"; got != want {
		t.Errorf("unexpected decoder: got %s, want %s", got, want)
	}

	if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 1596 || h != 1064 {
		t.Errorf("unexpected decoded image size: got %dx%d, want 1596x1064", w, h)
	}
}

func BenchmarkSafeEncoding(b *testing.B) {
	err := initLib()
	if err != nil {
		b.Fatal(err)
	}
	benchEncoding(b, true)
}

func BenchmarkRegularEncoding(b *testing.B) {
	err := initLib()
	if err != nil {
		b.Fatal(err)
	}
	benchEncoding(b, false)
}

func benchEncoding(b *testing.B, safe bool) {
	b.Helper()

	currentSetting := SafeEncoding
	defer func() {
		SafeEncoding = currentSetting
	}()
	SafeEncoding = safe

	f, err := ioutil.ReadFile("testdata/camel.heic")
	if err != nil {
		b.Fatal(err)
	}
	r := bytes.NewReader(f)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err = Decode(r)
		if err != nil {
			b.Fatal(err)
		}

		r.Seek(0, io.SeekStart)
	}
}
