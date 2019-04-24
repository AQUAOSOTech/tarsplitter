package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"github.com/c2h5oh/datasize"
)

var input = flag.String("i", "", "input archive file for splitting, OR input directory for archiving")
var command = flag.String("m", "split", "input mode command - must be 'split' or 'archive'")
var output = flag.String("o", "", "output path or folder")
var partCount = flag.Int64("p", 4, "number of parts to split the archive into, or number of threads when archiving")
var fileList = flag.String("f", "", "optional list of files instead of input for archiving")
var partSize = flag.String("s", "", "approximate size of each part, e.g. '100MB', '4G'")


func fatalIf(err error, args ...interface{}) {
	if err != nil {
		fmt.Print(err)
		fmt.Print(args...)
		fmt.Print("\n")
		os.Exit(1)
	}
}

func helpCommand() {
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

	var file *os.File
	var err error
	var partSizeBytes int64

	var size datasize.ByteSize
	_ = size.UnmarshalText([]byte(*partSize))
	partSizeBytes = int64(size.Bytes())

	if partSizeBytes == 0 {
		partSizeBytes = math.MaxInt64
	}

	if *input == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(*input)
		fatalIf(err, "Failed statting input", *input)
		defer file.Close()

		// get the file size
		stat, err := file.Stat()
		fatalIf(err, "Failed statting input", *input)

		if *partSize == "" {
			partSizeBytes = stat.Size() / *partCount
			fmt.Println(stat.Name(), "is", stat.Size(), "bytes, splitting into", *partCount, "parts of",
				partSizeBytes, "bytes")
		} else {
			fmt.Println(stat.Name(), "is", stat.Size(), "bytes, splitting into archives of",
				partSizeBytes, "bytes each")
		}
	}

	// now we get to work
	var info *tar.Header
	newTarCounter := 0
	var byteCounter int64
	tr := tar.NewReader(file)
	filesProcessed := 0

	var bytesBeforeWrite int64
	var bytesAfterWrite int64
	var tempInfo os.FileInfo

	nextArchive, err := filepath.Abs(fmt.Sprintf("%s%d.tar", *output, newTarCounter))
		fatalIf(err, "Something is not quite right with the output path")
	newTarFile, err := os.Create(nextArchive)
	fatalIf(err, "Failed opening new tar part", newTarFile)
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
		fatalIf(err, "Critical failure while reading tar file")

		// add the file from the original archive to the new archive
		tempInfo, _ = newTarFile.Stat()
		bytesBeforeWrite = tempInfo.Size()

		if (bytesBeforeWrite + info.Size > partSizeBytes || byteCounter > partSizeBytes) {
			byteCounter = 0
			newTarCounter++

			fatalIf(newTar.Close())
			fatalIf(newTarFile.Close())

			nextPath := fmt.Sprintf("%s%d.tar", *output, newTarCounter)
			nextArchive, err = filepath.Abs(nextPath)
			fatalIf(err, "new archive output path failed to initialize", nextPath)

			newTarFile, err = os.Create(nextArchive)
			fatalIf(err, "Failed opening new tar part", nextPath)

			newTar = tar.NewWriter(newTarFile)

			fmt.Println("Initialized next tar archive", newTarFile.Name())
		} else {
			err = newTar.WriteHeader(info)
			fatalIf(err, "failed writing header between tars")

			// write from the reader
			_, err = io.Copy(newTar, tr)
			fatalIf(err, "failed writing file body between tars")

			filesProcessed++
			if filesProcessed%10000 == 0 {
				fmt.Println("Processed files=", filesProcessed)
			}

			tempInfo, _ = newTarFile.Stat()
			bytesAfterWrite = tempInfo.Size()

			byteCounter += bytesAfterWrite - bytesBeforeWrite
		}
	}
}

const tarEndByteSpace = 1024

func doArchive() {
	if (*input == "" && *fileList == "") || *partCount <= 0 {
		fmt.Println("archive creates a tar archive using multithreading tricks")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var matches []string
	if *fileList != "" {
		matchText, err := ioutil.ReadFile(*fileList)
		fatalIf(err)
		matches = strings.Split(string(matchText), "\n")
	} else {
		var lm int
		// we must walk the directory, because Glob() fails at millions of files - "argument list too long"
		archiveErr := filepath.Walk(*input, func(path string, info os.FileInfo, err error) error {
			fatalIf(err, "walk failure", path)
			matches = append(matches, path)
			lm = len(matches)
			if lm % 100000 == 0 {
				fmt.Println("walking dir at", lm)
			}
			return nil
		})
		fatalIf(archiveErr, "tarsplitter walking input directory", *input)
		matchFileSaveText := []byte(strings.Join(matches, "\n"))
		fatalIf(ioutil.WriteFile(*output + ".txt", matchFileSaveText, os.ModePerm))
	}

	// open final file early so we don't realize it is a bad path after doing a bunch of work
	finalFile, archiveErr := os.Create(*output)
	fatalIf(archiveErr, "failed creating final archive", *output, archiveErr)
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
			fatalIf(err, "failed creating tar file", tarPath)

			newTar := tar.NewWriter(newTarFile)

			fmt.Println("Now writing", len(fileList), "files to", tarPath)

			var lastChar string
			for _, filename := range fileList {
				if filename == "" || filename == "." {
					continue
				}
				lastChar = filename[len(filename)-1:]
				if lastChar == "/" || lastChar == "\\" {
					fmt.Println("skipping", filename, tarPath)
					continue
				}
				file, err = os.Open(filename)
				fatalIf(err, "failed opening read file", tarPath, filename)

				stats, err = file.Stat()
				fatalIf(err, "failed statting open file", tarPath, filename)
				hdr = &tar.Header{
					Name: file.Name(),
					Mode: int64(stats.Mode()),
					Size: stats.Size(),
				}
				err = newTar.WriteHeader(hdr)
				fatalIf(err, "failed writing header between tars", tarPath, filename)

				_, err = io.Copy(newTar, file)
				fatalIf(err, "failed writing file body to tar", tarPath, filename)

				fatalIf(file.Close())
			}

			fatalIf(newTar.Close())
			fatalIf(newTarFile.Close())

			wg.Done()
		}(fileGroups[i], tempTarPaths[i])
	}

	wg.Wait()

	fmt.Println("created separate archives - now combining")

	var byteCount int64 = 0
	for i := 0; i < int(*partCount); i++ {
		inName := tempTarPaths[i]
		in, err := os.Open(inName)
		fatalIf(err, "failed to open input archive file for reading", inName)

		n, err := io.Copy(finalFile, in)
		fatalIf(err, "failed to append input archive to final", inName)
		// the tar spec has a bunch of empty bytes signifying the end of the archive, so we
		// want to remove those before writing the next archive
		byteCount += n
		fmt.Printf("wrote %d bytes of %s\n", n, inName)
		byteCount -= tarEndByteSpace // go back
		fatalIf(finalFile.Truncate(byteCount))
		fatalIf(finalFile.Sync())
		_, err = finalFile.Seek(0, 2) // put cursor back at the end
		fatalIf(err)

		// Delete the old input file
		fatalIf(in.Close())
		fatalIf(os.Remove(inName))
	}
}
