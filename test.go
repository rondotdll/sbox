package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var docker *client.Client

type sysinfo struct {
	base      string
	flavor    string
	version   string
	adjacents []string
}

func randString(length int) string {
	s := "qwertyuiopasdfghjklzxcvbnm1234567890QWERTYUIOPASDFGHJKLZXCVBNM"
	output := make([]byte, length) // (cstr of length)
	for i := 0; i < length; i++ {
		output[i] = s[rand.Intn(len(s)-1)]
	}
	return string(output)
}

func fetchSystemInfo() (sysinfo, error) {
	output := sysinfo{
		base:      "",
		flavor:    "",
		version:   "",
		adjacents: []string{},
	}

	var lines []byte // satisfy the compiler

	// account for windows? might end up removing support
	if runtime.GOOS == "windows" {
		output.base = "windows"
		goto ret
	}

	if buf, err := os.ReadFile("/etc/os-release"); err != nil {
		return output, fmt.Errorf("[!!!] failed to read OS from /etc/os-release: \n\t%w", err)
	} else {
		lines = buf
	}

	output.base = "linux"

	for _, line := range strings.Split(string(lines), "\n") {
		if strings.HasPrefix(line, "ID=") {
			output.flavor = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			output.version = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			output.adjacents = strings.Split(strings.Split(line, "=")[1], " ")
		}
	}

ret:
	return output, nil

}

// TODO:
// 1. Copy binary to container
// 2. Finish implementing dependency mapping
// 3. Add function to copy dependencies to container
// * also remember to mimic host paths in container (and maybe permissions?)
func main() {

	if len(os.Args) < 3 {
		fmt.Println("Usage: sbox <command> <exec> [args...]")
		os.Exit(-1)
	}

	command := os.Args[1]
	binary := os.Args[2]
	//args := os.Args[3:]

	switch command {
	case "run":
		break
	case "find":
		str, err := exec.LookPath(binary)
		if err != nil {
			fmt.Println("[!!!] exec", binary, "not found")
			os.Exit(-1)
		}
		fmt.Println(str)
		os.Exit(0)
	}

	if d, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err != nil {
		panic(err)
	} else {
		docker = d
	}

	hostSysinfo, err := fetchSystemInfo()
	if err != nil {
		panic(err)
	}

	hostContext := context.Background()

	containerConfig := &container.Config{
		Image: hostSysinfo.flavor + ":latest",
		Cmd:   []string{"strace", "", "echo", "Hello, world!"},
		Tty:   true,
	}

	// Configure host settings: for example, bind-mount the /lib directory from the host.
	systemConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   "/lib", // Host directory
				Target:   "/lib", // Container directory (same absolute path)
				ReadOnly: true,
			},
		},
		PortBindings: nat.PortMap{
			"8000/tcp": {
				{
					HostIP:   "0.0.0.0",
					HostPort: "8000",
				},
			},
		},
	}

	// (Optional) Network configuration if you need custom networking.
	netConfig := &network.NetworkingConfig{}

	fmt.Println("Pulling container image...")
	fmt.Printf("Host flavor is %s, pulling image %s:latest\n", hostSysinfo.flavor, hostSysinfo.flavor)
	output, err := docker.ImagePull(
		hostContext,
		hostSysinfo.flavor+":latest",
		image.PullOptions{},
	)
	if err != nil {
		// check if error is permission denied
		if strings.Contains(err.Error(), "permission denied") {
			fmt.Println("[!!!] Permission denied while pulling image. (Try running as root?)")
			os.Exit(-1)
		}
		panic(err)
	}
	io.Copy(os.Stdout, output)

	// Read output line by line
	//scanner := bufio.NewScanner(stderr)
	//for scanner.Scan() {
	//	line := scanner.Text()
	//
	//	// Skip lines with errors that aren't file paths
	//	if strings.Contains(line, "ENOENT") || strings.Contains(line, "EACCES") || strings.Contains(line, "ETXTBSY") {
	//		continue
	//	}
	//
	//	// Extract file path
	//	matches := re.FindAllStringSubmatch(line, -1)
	//	for _, match := range matches {
	//		if len(match) > 1 {
	//			paths[match[1]] = struct{}{}
	//		}
	//	}
	//}

	fmt.Println("Creating container...")
	cName := fmt.Sprintf("%s_sandbox_%s", hostSysinfo.flavor, randString(16))
	ccResponse, err := docker.ContainerCreate(hostContext, containerConfig, systemConfig, netConfig, nil, cName)
	if err != nil {
		panic(err)
	}
	fmt.Println("Created container", ccResponse.ID)
	fmt.Println("Container", cName, "created successfully.")

	fmt.Println("Starting container...")
	err = docker.ContainerStart(hostContext, ccResponse.ID, container.StartOptions{})
	if err != nil {
		panic(err)
	}

}
