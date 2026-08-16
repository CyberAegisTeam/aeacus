#!/bin/sh
set -e

rand() { xxd -l 64 -c 64 -p /dev/urandom; }
replace() {
	if [ "$(uname -s)" = "Darwin" ]; then
		sed -i '' "s/$2/$3/g" "$1"
	else
		sed -i "s/$2/$3/g" "$1"
	fi
}

hashVal=$(rand)
byteKey=$(rand | sed 's/\(..\)/0x\1, /g')
randomBytes=$(rand | sed 's/\(..\)/0x\1, /g')

for file in crypto.go studio/crypto.go; do
	if [ -f "$file.bak" ]; then mv "$file.bak" "$file"; fi
	cp "$file" "$file.bak"
done

replace crypto.go "HASH_HERE" "$hashVal"
replace crypto.go "0x01" "`echo $byteKey | head -c 382`"
replace crypto.go "{1}" "{`echo $randomBytes | head -c 382`}"
replace studio/crypto.go "HASH_HERE" "$hashVal"
replace studio/crypto.go "0x01" "`echo $byteKey | head -c 382`"

echo "Generated random keys for crypto.go"
