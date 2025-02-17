package sbox

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
)

func RandString(length int) string {
	s := "qwertyuiopasdfghjklzxcvbnm1234567890QWERTYUIOPASDFGHJKLZXCVBNM"
	output := make([]byte, length) // (cstr of length)
	for i := 0; i < length; i++ {
		output[i] = s[rand.Intn(len(s)-1)]
	}
	return string(output)
}

func FetchSystemInfo() (Sysinfo, error) {
	output := Sysinfo{
		Base:      "",
		Flavor:    "",
		Version:   "",
		Adjacents: []string{},
	}

	var lines []byte // satisfy the compiler

	// account for windows? might end up removing support
	if runtime.GOOS == "windows" {
		output.
			Base = "windows"
		goto ret
	}

	if buf, err := os.ReadFile("/etc/os-release"); err != nil {
		return output, fmt.Errorf("[!!!] failed to read OS from /etc/os-release: \n\t%w", err)
	} else {
		lines = buf
	}

	output.Base = "linux"

	for _, line := range strings.Split(string(lines), "\n") {
		if strings.HasPrefix(line, "ID=") {
			output.Flavor = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			output.Version = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			output.Adjacents = strings.Split(strings.Split(line, "=")[1], " ")
		}
	}

ret:
	return output, nil

}
