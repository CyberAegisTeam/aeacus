#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
version=3.0.0
dist="dist/aeacus-$version"

restore_crypto() {
	[ ! -f crypto.go.bak ] || mv crypto.go.bak crypto.go
	[ ! -f studio/crypto.go.bak ] || mv studio/crypto.go.bak studio/crypto.go
}
trap restore_crypto EXIT INT TERM

rm -rf "$dist"
mkdir -p "$dist/personal" "$dist/development/windows" "$dist/development/linux"

echo "Generating matching Aeacus 3.0.0 configuration keys..."
sh misc/dev/gen-crypto.sh

echo "Building Windows Personal Studio..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags desktop,production -ldflags "-s -w -H windowsgui -X main.defaultMode=personal" -o "$dist/personal/Aeacus-Studio-Personal-Windows.exe" ./studio

if [ "$(uname -s)" = "Darwin" ]; then
	echo "Building native macOS Personal Studio..."
	mkdir -p "$dist/personal/Aeacus Studio Personal.app/Contents/MacOS" "$dist/personal/Aeacus Studio Personal.app/Contents/Resources"
	cp packaging/macos/Info.plist "$dist/personal/Aeacus Studio Personal.app/Contents/Info.plist"
	cp packaging/macos/AeacusStudio.icns "$dist/personal/Aeacus Studio Personal.app/Contents/Resources/AeacusStudio.icns"
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -tags desktop,production -ldflags "-s -w -X main.defaultMode=personal -extldflags '-framework UniformTypeIdentifiers'" -o "$dist/personal/Aeacus Studio Personal.app/Contents/MacOS/aeacus-studio-personal" ./studio
	codesign --force --deep --sign - --timestamp=none "$dist/personal/Aeacus Studio Personal.app"
	codesign --verify --deep --strict --verbose=2 "$dist/personal/Aeacus Studio Personal.app"
	(cd "$dist/personal" && zip -qr Aeacus-Studio-Personal-macOS.zip "Aeacus Studio Personal.app")
	dmg_stage=$(mktemp -d)
	cp -R "$dist/personal/Aeacus Studio Personal.app" "$dmg_stage/"
	ln -s /Applications "$dmg_stage/Applications"
	hdiutil create -quiet -volname "Aeacus Studio Personal" -srcfolder "$dmg_stage" -ov -format UDZO "$dist/personal/Aeacus-Studio-Personal-macOS.dmg"
fi

if [ "$(uname -s)" = "Linux" ]; then
	echo "Building native Linux Personal Studio..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -tags desktop,production,webkit2_41 -ldflags "-s -w -X main.defaultMode=personal" -o "$dist/personal/aeacus-studio-personal-linux-amd64" ./studio
fi

echo "Building Windows Development Studio bundle..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags desktop,production -ldflags "-s -w -H windowsgui -X main.defaultMode=development" -o "$dist/development/windows/Aeacus-Studio-Development.exe" ./studio
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$dist/development/windows/aeacus.exe" .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags phocus -ldflags "-s -w" -o "$dist/development/windows/phocus.exe" .
cp -R assets misc "$dist/development/windows/"
printf '%s\n' 'AEACUS_STUDIO_DEVELOPMENT_BUNDLE_V3' > "$dist/development/windows/.aeacus-development-bundle"

if [ "$(uname -s)" = "Linux" ]; then
	echo "Building native Linux Development Studio bundle..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -tags desktop,production,webkit2_41 -ldflags "-s -w -X main.defaultMode=development" -o "$dist/development/linux/aeacus-studio-development" ./studio
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$dist/development/linux/aeacus" .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags phocus -ldflags "-s -w" -o "$dist/development/linux/phocus" .
	chmod +x "$dist/development/linux/aeacus-studio-development" "$dist/development/linux/aeacus" "$dist/development/linux/phocus"
	cp -R assets misc "$dist/development/linux/"
	printf '%s\n' 'AEACUS_STUDIO_DEVELOPMENT_BUNDLE_V3' > "$dist/development/linux/.aeacus-development-bundle"
	tar -C "$dist/development" -czf "$dist/development/Aeacus-Studio-Development-Linux.tar.gz" linux
fi

cp README.md docs/AEACUS_3_RELEASE_GUIDE.md "$dist/"

(cd "$dist/development" && zip -qr Aeacus-Studio-Development-Windows.zip windows)

echo "Aeacus $version release created at $dist"
