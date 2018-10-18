build:
		rm -rf build
		mkdir -p build/linux
		mkdir -p build/mac
		mkdir -p build/windows

		env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o build/linux/tarsplitter
		env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o build/mac/tarsplitter
		env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o build/windows/tarsplitter.exe

		chmod +x build/linux/tarsplitter
		chmod +x build/mac/tarsplitter
		chmod +x build/windows/tarsplitter.exe
.PHONY: build
