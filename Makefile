.DEFAULT_GOAL := all

.PHONY: release-v3 test-v3

release-v3:
	sh scripts/build-release-v3.sh

test-v3:
	go test ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/aeacus-linux .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags phocus -o /tmp/phocus-linux .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/aeacus-windows.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags phocus -o /tmp/phocus-windows.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags desktop,production -ldflags '-H windowsgui -X main.defaultMode=personal' -o /tmp/aeacus-studio-windows.exe ./studio
	@if [ "$$(uname -s)" = "Darwin" ]; then CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -tags desktop,production -ldflags '-X main.defaultMode=personal -extldflags "-framework UniformTypeIdentifiers"' -o /tmp/aeacus-studio-macos ./studio; fi
.SILENT: release-lin release-win release

all:
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -tags phocus -o ./phocus.exe . && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -o ./aeacus.exe . && \
	echo "Windows production build successful!" && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o ./aeacus . && \
	echo "Linux production build successful!"

all-dev:
	CGO_ENABLED=0 GOOS=windows go build -tags phocus -o ./phocus.exe . && \
	CGO_ENABLED=0 GOOS=windows go build -o ./aeacus.exe . && \
	echo "Windows development build successful!" && \
	CGO_ENABLED=0 GOOS=linux go build -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -o ./aeacus . && \
	echo "Linux development build successful!"

lin:
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o ./aeacus . && \
	echo "Linux production build successful!"

lin-dev:
	CGO_ENABLED=0 GOOS=linux go build -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -o ./aeacus . && \
	echo "Linux development build successful!"

win:
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -tags phocus -o ./phocus.exe . && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -o ./aeacus.exe . && \
	echo "Windows production build successful!"

win-dev:
	CGO_ENABLED=0 GOOS=windows go build -tags phocus -o ./phocus.exe . && GOOS=windows go build -o ./aeacus.exe . && \
	echo "Windows development build successful!"

release:
	echo "Building obfuscated binaries..." && \
	sh misc/dev/gen-crypto.sh && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -tags phocus -o ./phocus.exe . && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -o ./aeacus.exe . && \
	echo "Windows production build successful!" && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o ./aeacus . && \
	echo "Linux production build successful!" && \
	mv crypto.go.bak crypto.go && \
	echo "Restored crypto.go" && \
	mkdir aeacus-win32/ && mkdir aeacus-linux/ && \
	mv aeacus.exe aeacus-win32/aeacus.exe && \
	mv phocus.exe aeacus-win32/phocus.exe && \
	mv aeacus aeacus-linux/aeacus && \
	mv phocus aeacus-linux/phocus && \
	cp -Rf assets/ aeacus-win32/ && \
	cp -Rf misc/ aeacus-win32/ && \
	cp -Rf LICENSE aeacus-win32/ && \
	cp -Rf assets/ aeacus-linux/ && \
	cp -Rf misc/ aeacus-linux/ && \
	cp -Rf LICENSE aeacus-linux/ && \
	zip -r aeacus-win32.zip aeacus-win32/ > /dev/null && \
	echo "Successfully compressed aeacus-win32!" && \
	zip -r aeacus-linux.zip aeacus-linux/ > /dev/null && \
	echo "Successfully compressed aeacus-linux!" && \
	rm -rf aeacus-win32/ && rm -rf aeacus-linux/

release-lin:
	echo "Building obfuscated binaries..." && \
	sh misc/dev/gen-crypto.sh && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -tags phocus -o ./phocus . && \
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o ./aeacus . && \
	echo "Linux production build successful!" && \
	mv crypto.go.bak crypto.go && \
	echo "Restored crypto.go" && \
	mkdir aeacus-linux/ && \
	mv aeacus aeacus-linux/aeacus && \
	mv phocus aeacus-linux/phocus && \
	cp -Rf assets/ aeacus-linux/ && \
	cp -Rf misc/ aeacus-linux/ && \
	cp -Rf LICENSE aeacus-linux/ && \
	zip -r aeacus-linux.zip aeacus-linux/ > /dev/null && \
	echo "Successfully compressed aeacus-linux!" && \
	rm -rf aeacus-linux/

release-win:
	echo "Building obfuscated binaries..." && \
	sh misc/dev/gen-crypto.sh && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -tags phocus -o ./phocus.exe . && \
	CGO_ENABLED=0 GOOS=windows go build -ldflags '-s -w' -o ./aeacus.exe . && \
	echo "Windows production build successful!" && \
	mv crypto.go.bak crypto.go && \
	echo "Restored crypto.go" && \
	mkdir aeacus-win32/ && \
	mv aeacus.exe aeacus-win32/aeacus.exe && \
	mv phocus.exe aeacus-win32/phocus.exe && \
	cp -Rf assets/ aeacus-win32/ && \
	cp -Rf misc/ aeacus-win32/ && \
	cp -Rf LICENSE aeacus-win32/ && \
	zip -r aeacus-win32.zip aeacus-win32/ > /dev/null && \
	echo "Successfully compressed aeacus-win32!" && \
	rm -rf aeacus-win32/
