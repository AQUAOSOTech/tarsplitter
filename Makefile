build:
		rm -rf build
		mkdir build

		env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o build/tarsplitter_linux
		env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o build/tarsplitter_mac
		env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o build/tarsplitter.exe

		chmod +x build/tarsplitter_linux
		chmod +x build/tarsplitter_mac
.PHONY: build
