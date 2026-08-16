//go:build darwin

package main

// The competition runtime remains Windows/Linux-only. These compatibility
// implementations allow the shared parser, crypto, and check tests to run on
// macOS, where Aeacus Studio Personal is supported.

import (
	"crypto/md5"
	"io"
	"os"
	"os/exec"
)

func readFile(name string) (string, error)      { data, err := os.ReadFile(name); return string(data), err }
func decodeString(value string) (string, error) { return value, nil }
func rawCmd(command string) *exec.Cmd           { return exec.Command("/bin/sh", "-c", command) }
func checkTrace()                               {}
func sendNotification(string)                   {}
func playAudio(string)                          {}
func adminCheck() bool                          { return true }
func getInfo(string)                            { warn("Aeacus runtime information gathering is not supported on macOS") }
func launchIDPrompt()                           { warn("The production Team ID prompt is not supported on macOS") }
func writeDesktopFiles()                        { warn("Image release is not supported on macOS") }
func configureAutologin()                       { warn("Image release is not supported on macOS") }
func installFont()                              { warn("Image release is not supported on macOS") }
func installService()                           { warn("Image release is not supported on macOS") }
func cleanUp()                                  { warn("Image release is not supported on macOS") }
func hashFileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hexEncode(string(hash.Sum(nil))), nil
}
