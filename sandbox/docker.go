package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var docker *client.Client

func init() {
	if d, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err != nil {
		panic(err)
	} else {
		docker = d
	}
}

// Copy file from the host to the container
func CopyToContainer(box *Sandbox, src, dst string) error {
	ctx := context.Background()

	// copy the file to the container
	f, e := os.Open(src)
	if e != nil {
		return e
	}
	defer f.Close()

	return docker.CopyToContainer(ctx, box.ID, dst, f, container.CopyToContainerOptions{})
}

func ResolveDependencies(box *Sandbox, command []string) error {
	ctx := context.Background()

	execRes, err := docker.ContainerExecCreate(ctx, box.ID, container.ExecOptions{
		Cmd:          append([]string{"strace", "-e", "trace=open,openat,access,stat"}, command...),
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		return err
	}

	resolved := false

	for !resolved {
		// attach to the exec instance
		attachResp, err := docker.ContainerExecAttach(ctx, execRes.ID, container.ExecAttachOptions{
			Tty: false,
		})
		if err != nil {
			return fmt.Errorf("failed to attach to exec instance: %w", err)
		}

		defer attachResp.Close()

		// Capture output
		var stdoutBuf, stderrBuf bytes.Buffer
		// stdcopy.StdCopy demultiplexes the stream into stdout and stderr
		if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader); err != nil && err != io.EOF {
			return fmt.Errorf("failed to read output: %w", err)
		}

		// Optionally, poll to ensure the process has finished
		inspect := container.ExecInspect{}

		// poll the process for 30 seconds
		for i := 0; i < 30; i++ {
			inspect, err = docker.ContainerExecInspect(ctx, execRes.ID)
			if err != nil {
				return err
			}
			if !inspect.Running {
				break
			}

			time.Sleep(time.Second) // pause for 1 second
		}
		// if the container is still running after 30 seconds, assume dependencies have been resolved
		if inspect.Running {
			log.Println("Container still running after 30 seconds... Assuming dependencies resolved?")
			if err := docker.ContainerKill(ctx, box.ID, "SIGKILL"); err != nil {
				return err
			}
		}

		resolved = true
		if inspect.ExitCode != 0 {
			log.Println("Process exited with non-zero exit code... Assuming missing dependencies.")
			log.Println("Parsing strace output...")
			cout := append(strings.Split(stdoutBuf.String(), "\n\r"), strings.Split(stderrBuf.String(), "\n\r")...)
			for _, line := range cout {
				if strings.Contains(line, "ENOENT") {
					x := strings.Split(line, ", ")[1]
					if strings.HasPrefix(x, "\"") && strings.HasSuffix(x, "\"") {
						hdep := strings.ReplaceAll(x, "\"", "")
						cdep := hdep

						// verify dependency is a static path
						if !strings.HasPrefix(hdep, "/") {
							hostpath, e := exec.LookPath(box.Command[0])
							if e != nil {
								return e
							}
							hdep = hostpath + hdep
							cdep = "/usr/bin/" + cdep
						}

						log.Println("Found missing dependency ", line)
						log.Println("?> Attempting to resolve...")
						if _, e := os.Open(hdep); e == os.ErrNotExist {
							return fmt.Errorf("Dependency not found on host: %s", cdep)
						}
						log.Println("?> Found similar on host. Copying to container...")
						if e := CopyToContainer(box, hdep, cdep); e != nil {
							return e
						}
						log.Println("?> Done.")
					}
				}
			}
			continue
		}
		resolved = true
		log.Println("Finished resolving dependencies, Sandbox ready.")
		return nil
	}

	return nil
}

// Create a new Sandbox (and corresponding docker container) with the given command
func NewSandbox(cmd []string) (*Sandbox, error) {
	var output *Sandbox
	ctx := context.Background()

	host, err := FetchSystemInfo()
	if err != nil {
		return nil, err
	}

	log.Println("Fetching image \"", host.Flavor+":latest\"")
	// make sure the image we're targeting exists on the host
	image_pull_log, err := docker.ImagePull(
		ctx,
		host.Flavor+":latest",
		image.PullOptions{},
	)
	if err != nil {
		// Handle the special case where user forgot to run sudo
		if strings.Contains(err.Error(), "permission denied") {
			fmt.Println("Permission denied while pulling image. (Try running as root?)")
			os.Exit(-1)
		}
		return nil, err
	}
	// dump the docker pull log to log
	_, e := io.Copy(log.Writer(), image_pull_log)
	if e != nil {
		return nil, e
	}

	output.Name = fmt.Sprintf("sandbox_", RandString(16))

	log.Println("Creating container...")
	cc_res, e := docker.ContainerCreate(ctx,
		// general host config info
		&container.Config{
			Image: host.Flavor + ":latest",
			Tty:   true,
		},
		// port configs
		&container.HostConfig{
			//PortBindings: nat.PortMap{
			//	"80/tcp": {
			//		{
			//			HostIP:   "0.0.0.0",
			//			HostPort: "80",
			//		},
			//	},
			//	"443/tcp": {
			//		{
			//			HostIP:   "0.0.0.0",
			//			HostPort: "443",
			//		},
			//	},
			//},
		},
		&network.NetworkingConfig{},
		nil,
		output.Name,
	)
	// if we can't create the container, just panic
	if e != nil {
		panic(e)
	}

	binLocation, e := exec.LookPath(cmd[0])
	if e != nil {
		return nil, e
	}
	log.Println("Copying binary into container...")
	if e := CopyToContainer(output, binLocation, "/usr/bin/"+cmd[0]); e != nil {
		return nil, e
	}

	output.ID = cc_res.ID
	output.Command = cmd
	log.Println("Done creating container " + output.Name)
	return output, nil
}
