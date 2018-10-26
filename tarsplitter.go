package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
)

var input = flag.String("i", "", "input file or folder")
var command = flag.String("m", "split", "input mode command - must be 'split' or 'archive'")
var output = flag.String("o", "", "output path or folder")
var partCount = flag.Int64("p", 4, "number of parts to split the archive into, or number of threads when archiving")

func fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}

func helpCommand() {
	fmt.Println("first argument must be 'split' or 'archive'")
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	flag.Parse()

	if *command == "split" {
		doSplit()
	} else if *command == "archive" {
		doArchive()
	} else {
		helpCommand()
	}

	fmt.Println("All done")
}

func doSplit() {
	if *input == "" || *partCount <= 0 {
		fmt.Println("splitter splits a tar archive into approximately equal parts")
		flag.PrintDefaults()
		os.Exit(1)
	}

	file, err := os.Open(*input)
	if err != nil {
		fatal("Failed statting input", *input, err)
	}
	defer file.Close()

	// get the file size
	stat, err := file.Stat()
	if err != nil {
		fatal("Failed statting input", *input, err)
	}

	partSizeBytes := stat.Size() / *partCount
	fmt.Println(stat.Name(), "is", stat.Size(), "bytes, splitting into", *partCount, "parts of",
		partSizeBytes, "bytes")

	// now we get to work
	var info *tar.Header
	newTarCounter := 0
	var byteCounter int64
	tr := tar.NewReader(file)
	filesProcessed := 0

	var bytesBeforeWrite int64
	var bytesAfterWrite int64
	var tempInfo os.FileInfo

	p, err := filepath.Abs(fmt.Sprintf("%s%d.tar", *output, newTarCounter))
	if err != nil {
		fatal("Something is not quite right with the output path", err)
	}
	newTarFile, err := os.Create(p)
	if err != nil {
		fatal("Failed opening new tar part", err)
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

		// add the file from the original archive to the new archive
		tempInfo, _ = newTarFile.Stat()
		bytesBeforeWrite = tempInfo.Size()
		if err = newTar.WriteHeader(info); err != nil {
			fatal("failed writing header between tars", err)
		}
		if _, err = io.Copy(newTar, tr); err != nil {
			fatal("failed writing file body between tars", err)
		}

		filesProcessed++
		if filesProcessed%10000 == 0 {
			fmt.Println("Processed files=", filesProcessed)
		}

		tempInfo, _ = newTarFile.Stat()
		bytesAfterWrite = tempInfo.Size()

		byteCounter += bytesAfterWrite - bytesBeforeWrite

		if byteCounter > partSizeBytes {
			byteCounter = 0
			newTarCounter++

			err = newTar.Close()
			if err != nil {
				fatal("failed closing tar writer", err)
			}
			err = newTarFile.Close()
			if err != nil {
				fatal("failed closing tar file", err)
			}
			p, err = filepath.Abs(fmt.Sprintf("%s%d.tar", *output, newTarCounter))
			if err != nil {
				fatal("new archive output path failed to initialize", err)
			}
			newTarFile, err = os.Create(p)
			if err != nil {
				fatal("Failed opening new tar part", err)
			}
			newTar = tar.NewWriter(newTarFile)

			fmt.Println("Initialized next tar archive", newTarFile.Name())
		}
	}
}

func doArchive() {
	if *input == "" || *partCount <= 0 {
		fmt.Println("archive creates a tar archive using multithreading tricks")
		flag.PrintDefaults()
		os.Exit(1)
	}

	matches, archiveErr := filepath.Glob(*input)
	if archiveErr != nil {
		fatal("Failed statting input", *input, archiveErr)
	}

	// open final file early so we don't realize it is a bad path after doing a bunch of work
	finalFile, archiveErr := os.Create(*output)
	if archiveErr != nil {
		fatal("failed creating final archive", *output, archiveErr)
	}
	archiveErr = finalFile.Close()
	if archiveErr != nil {
		fatal("failed pre-closing final archive", *output, archiveErr)
	}
	// appendonly for speed
	finalFile, archiveErr = os.OpenFile(*output, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if archiveErr != nil {
		fatal("Failed opening final archive input", *output, finalFile, archiveErr)
	}
	defer finalFile.Close()

	fmt.Println("matched", len(matches), "files")

	var tempTarPaths []string
	for i := 0; i < int(*partCount); i++ {
		tempTarPaths = append(tempTarPaths, fmt.Sprintf("%s%d.tar", *output, i))
	}
	var fileGroups [][]string
	perGroup := int64(math.Ceil(float64(len(matches)) / float64(*partCount)))
	var f int64 = 0
	for i := 0; i < int(*partCount); i++ {
		end := int(math.Min(float64(len(matches)), float64((perGroup*int64(i))+perGroup)))
		fg := matches[f:end]
		fileGroups = append(fileGroups, fg)
		f += perGroup
	}

	var wg sync.WaitGroup
	for i := 0; i < len(fileGroups); i++ {
		wg.Add(1)
		go func(fileList []string, tarPath string) {
			var err error
			var file *os.File
			var stats os.FileInfo
			var hdr *tar.Header
			newTarFile, err := os.Create(tarPath)
			if err != nil {
				fatal("failed creating tar file", tarPath, err)
			}
			newTar := tar.NewWriter(newTarFile)
			fmt.Println("Now writing", len(fileList), "files to", tarPath)
			for _, filename := range fileList {
				file, err = os.Open(filename)
				if err != nil {
					fatal("failed opening read file", tarPath, filename, err)
				}
				stats, err = file.Stat()
				if err != nil {
					fatal("failed statting open file", tarPath, filename, err)
				}
				hdr = &tar.Header{
					Name: file.Name(),
					Mode: int64(stats.Mode()),
					Size: stats.Size(),
				}
				if err = newTar.WriteHeader(hdr); err != nil {
					fatal("failed writing header between tars", tarPath, filename, err)
				}
				if _, err = io.Copy(newTar, file); err != nil {
					fatal("failed writing file body to tar", tarPath, filename, err)
				}

				err = file.Close()
				if err != nil {
					fatal("failed closing read file", tarPath, filename, err)
				}
			}
			err = newTar.Close()
			if err != nil {
				fatal("did not close tar writer", tarPath, err)
			}
			newTarFile.Close()
			if err != nil {
				fatal("did not close tar file", tarPath, err)
			}
			wg.Done()
		}(fileGroups[i], tempTarPaths[i])
	}

	wg.Wait()

	fmt.Println("created separate archives - now combining")

	for i := 0; i < int(*partCount); i++ {
		inName := tempTarPaths[i]
		in, err := os.Open(inName)
		if err != nil {
			fatal("failed to open input archive file for reading", inName, err)
		}

		n, err := io.Copy(finalFile, in)
		if err != nil {
			log.Fatalln("failed to append input archive to final", inName, err)
		}
		log.Printf("wrote %d bytes of %s\n", n, inName)

		// Delete the old input file
		err = in.Close()
		if err != nil {
			fatal("failed closing input tmp archive", err)
		}
		//if err := os.Remove(inName); err != nil {
		//	log.Fatalln("failed to remove", inName)
		//}
	}
}
