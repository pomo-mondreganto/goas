package main

import (
	"github.com/corona10/goimagehash"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"image/jpeg"
	"os"
)

var (
	path = pflag.StringP("file", "f", "sample.jpg", "Path to JPEG image to hash")
)

func main() {
	pflag.Parse()
	file, err := os.Open(*path)
	if err != nil {
		logrus.Fatalf("Error opening file: %v", err)
	}

	img, err := jpeg.Decode(file)
	if err != nil {
		logrus.Fatalf("Error decoding JPEG: %v", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		logrus.Fatalf("Error calculating hash: %v", err)
	}

	logrus.Printf("Hash is %d\n", hash.GetHash())
}
