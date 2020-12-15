package main

import (
	"github.com/corona10/goimagehash"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"image/jpeg"
	"os"
)

var (
	paths = pflag.StringArrayP("files", "f", []string{"sample.jpg", "sample.jpg"}, "Paths of images to compare")
)

func main() {
	pflag.Parse()
	if len(*paths) != 2 {
		logrus.Fatalf("Pass 2 images to compare")
	}
	hash1 := calcHash((*paths)[0])
	hash2 := calcHash((*paths)[1])
	logrus.Printf("Hash for first file is %d, second: %d\n", hash1.GetHash(), hash2.GetHash())

	dist, err := hash1.Distance(hash2)
	if err != nil {
		logrus.Fatalf("Error calculating distance: %v", err)
	}
	logrus.Printf("Distance is %d", dist)
}

func calcHash(path string) *goimagehash.ImageHash {
	file, err := os.Open(path)
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
	return hash
}
