package rescontainerpodman

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"

	"github.com/opensvc/om3/drivers/rescontainerocibase"
)

type (
	podman struct {
	}

	containerStarter struct {
		ID string
	}
)

func (c *containerStarter) Start(ctx context.Context) error {
	p := podman{}
	return p.cmd(ctx, "start", c.ID).Run()
}

func (c *containerStarter) Wait(ctx context.Context, opts ...rescontainerocibase.WaitCondition) (int, error) {
	p := podman{}
	return p.Wait(ctx, c.ID, opts...)
}

func (p *podman) NewContainer(ctx context.Context, id string) (cs rescontainerocibase.ContainerStarter, err error) {
	return &containerStarter{ID: id}, nil
}

func (p *podman) Running(ctx context.Context, name string) (bool, error) {
	cmd := p.cmd(ctx, "container", "inspect", "-f", "{{.State.Running}}", name)
	if b, err := cmd.Output(); err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			if e.ExitCode() == 125 {
				return false, nil
			}
		}
		return false, err
	} else if bytes.Equal(b, []byte("true")) {
		return true, nil
	} else {
		return false, nil
	}
}

func (p *podman) Remove(ctx context.Context, name string) error {
	cmd := p.cmd(ctx, "container", "rm", name)
	return cmd.Run()
}

func (p *podman) Start(ctx context.Context, opts ...string) error {
	//args := []string{"container", "start", "--name", name}
	//args = append(args, opts...)
	return p.cmd(ctx, opts...).Run()
}

func (p *podman) Stop(ctx context.Context, name string) error {
	cmd := p.cmd(ctx, "container", "kill", name)
	return cmd.Run()
}

func (p *podman) Inspect(ctx context.Context, name string) (is rescontainerocibase.Inspecter, err error) {
	cmd := p.cmd(ctx, "container", "inspect", "--format", "{{json .}}", name)
	d := &InspectData{}
	if b, err := cmd.Output(); err != nil {
		if isNotFound(err) {
			return nil, rescontainerocibase.ErrNotFound
		} else {
			return nil, err
		}
	} else if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return d, nil
}

func (p *podman) Wait(ctx context.Context, name string, opts ...rescontainerocibase.WaitCondition) (int, error) {
	args := []string{"container", "wait", "--condition"}
	for _, v := range opts {
		args = append(args, string(v))
	}
	args = append(args, name)
	cmd := p.cmd(ctx, args...)
	err := cmd.Start()
	if err != nil {
		if isNotFound(err) {
			return 0, rescontainerocibase.ErrNotFound
		} else {
			return 0, err
		}
	}
	if b, err := cmd.Output(); err != nil {
		return 0, err
	} else if i, err := strconv.Atoi(string(b)); err != nil {
		return 0, err
	} else {
		return i, nil
	}
}

func (p *podman) Pull(ctx context.Context, opts ...string) error {
	return p.cmd(ctx, opts...).Run()
}

func (p *podman) HasImage(ctx context.Context, name string) (exists bool, err error) {
	cmd := p.cmd(ctx, "image", "inspect", name)
	if err := cmd.Run(); err != nil {
		if isNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	} else {
		return true, nil
	}
}

func (p *podman) Create(_ context.Context, _ rescontainerocibase.CreateOptions) error {
	return nil
	//cmd := p.cmd(ctx, "container", "--name", options.Name, options.Image)
	//return cmd.Run()
}

func (p *podman) PullOptions(bt *rescontainerocibase.BT) ([]string, error) {
	return []string{"image", "pull", bt.Image}, nil
}

func (p *podman) StartOptions(bt *rescontainerocibase.BT) ([]string, error) {
	opts := []string{"container", "run", "--name", bt.ContainerName()}
	for k, v := range bt.Labels() {
		opts = append(opts, "--label", k+"="+v)
	}
	if bt.Detach {
		opts = append(opts, "--detach")
	}
	if s, err := bt.FormatNS(bt.NetNS); err != nil {
		return opts, err
	} else if s == "" {
		opts = append(opts, "--net=none")
	} else {
		opts = append(opts, "--net", s)
	}
	opts = append(opts, bt.Image)
	return opts, nil
}

func (p *podman) cmd(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "podman", args...)
}

func isNotFound(err error) bool {
	switch e := err.(type) {
	case *exec.ExitError:
		if e.ExitCode() == 125 {
			return true
		}
	}
	return false
}
