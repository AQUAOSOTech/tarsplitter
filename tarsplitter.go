package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var input = flag.String("i", "", "input tar file to be split")
var output = flag.String("o", "", "output path")
var partCount = flag.Int64("p", 4, "number of equal parts to split the tar file into")

func main() {
	flag.Parse()
	if *input == "" || *partCount <= 0 {
		fmt.Println("splitter splits a tar archive into approximately equal parts")
		flag.PrintDefaults()
		os.Exit(1)
	}

	file, err := os.Open(*input)
	if err != nil {
		// handle the error here
		return
	}
	defer file.Close()

	// get the file size
	stat, err := file.Stat()
	if err != nil {
		log.Fatal("Failed statting input", *input, err)
	}

	partSizeBytes := stat.Size() / *partCount
	fmt.Println(stat.Name(), "is", stat.Size(), "bytes, splitting into", *partCount, "parts of",
		partSizeBytes, "bytes")

	// now we get to work
	var info *tar.Header
	newTarCounter := 0
	var byteCounter int64
	tr := tar.NewReader(file)
	var contents []byte
	filesProcessed := 0

	p, err := filepath.Abs(fmt.Sprintf("%s%d.tar", *output, newTarCounter))
	if err != nil {
		log.Fatal("Something is not quite right with the output path", err)
	}
	newTarFile, err := os.Create(p)
	if err != nil {
		log.Fatal("Failed opening new tar part", err)
	}
	newTar := tar.NewWriter(newTarFile)
	fmt.Println("First new archive is", newTarFile.Name())

	for {
		info, err = tr.Next()
		if err == io.EOF || info == nil {
			fmt.Println("Done reading input archive")
			newTar.Close()
			newTarFile.Close()
			break // End of archive
		}
		if err != nil {
			fmt.Println("Critical failure while reading tar file")
			fmt.Println(err)
			os.Exit(1)
		}

		contents, err = ioutil.ReadAll(tr)
		if err != nil {
			fmt.Println("failed while reading from original archive", info.Name, err)
			continue
		}

		// add the file from the original archive to the new archive
		if err := newTar.WriteHeader(info); err != nil {
			log.Fatal("failed writing header between tars", err)
		}
		if _, err := newTar.Write(contents); err != nil {
			log.Fatal("failed writing file body between tars", err)
		}

		filesProcessed++
		if filesProcessed % 10000 == 0 {
			fmt.Println("Processed files=", filesProcessed)
		}

		byteCounter += info.FileInfo().Size() + info.Size

		if byteCounter > partSizeBytes {
			byteCounter = 0
			newTarCounter++

			err = newTar.Close()
			if err != nil {
				log.Fatal("failed closing tar writer", err)
			}
			err = newTarFile.Close()
			if err != nil {
				log.Fatal("failed closing tar file", err)
			}
			p, err = filepath.Abs(fmt.Sprintf("%s%d.tar", *output, newTarCounter))
			if err != nil {
				log.Fatal("new archive output path failed to initialize", err)
			}
			newTarFile, err = os.Create(p)
			if err != nil {
				log.Fatal("Failed opening new tar part", err)
			}
			newTar = tar.NewWriter(newTarFile)

			fmt.Println("Initialized next tar archive", newTarFile.Name())
		}
	}

	fmt.Println("All done")
}
