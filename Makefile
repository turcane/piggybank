.DEFAULT_GOAL := build
version := 1.4

clean:
	$(RM) dist/piggybank dist/piggybank.exe
	rm -rf release

build: clean
	go build -o dist/piggybank piggybank.go

serve: build
	cd dist && ./piggybank

linux: clean
	mkdir release
	GOOS=linux GOARCH=amd64 go build -ldflags '-w -extldflags "-static"' -o dist/piggybank piggybank.go
	zip -rj release/PiggyBank_$(version)_Linux.zip dist/*
	$(RM) dist/piggybank dist/piggybank.exe

release: clean
	mkdir release

	# macOS x64
	GOOS=darwin GOARCH=amd64 go build -o dist/piggybank piggybank.go
	zip -rj release/PiggyBank_$(version)_macOS.zip dist/*
	$(RM) dist/piggybank dist/piggybank.exe
	
	# Windows x64
	GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o dist/piggybank.exe piggybank.go
	zip -rj release/PiggyBank_$(version)_Windows.zip dist/*
	$(RM) dist/piggybank dist/piggybank.exe
	
	# Linux x64
	GOOS=linux GOARCH=amd64 CC=x86_64-linux-musl-gcc go build -ldflags '-w -extldflags "-static"' -o dist/piggybank piggybank.go
	zip -rj release/PiggyBank_$(version)_Linux.zip dist/*
	$(RM) dist/piggybank dist/piggybank.exe 
	
	# ARMHF x86 (Raspberry Pi) 
	GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-musleabihf-gcc go build -o dist/piggybank piggybank.go
	zip -rj release/PiggyBank_$(version)_RaspberryPi.zip dist/*
	$(RM) dist/piggybank dist/piggybank.exe
