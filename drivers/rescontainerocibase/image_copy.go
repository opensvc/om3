package rescontainerocibase

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// engineStderrKept is how much of the end of the stderr of an engine command
// is kept for its error: its last line is what is read, and a command
// loading an archive may write for as long as it runs.
const engineStderrKept = 4096

// imageSaveArgs are the arguments of the engine command writing an image of
// its store on its stdout, as an archive carrying the name it is saved by:
// the load of the archive gives the image that name, which is the one a
// start looks for.
func imageSaveArgs(image string) []string {
	return []string{"image", "save", image}
}

// imageLoadArgs are the arguments of the engine command loading in its store
// the archive it reads on its stdin. The engines read the archive of each
// other.
func imageLoadArgs() []string {
	return []string{"image", "load"}
}

// imageRemoveArgs are the arguments of the engine command removing a name
// from an image of its store, and the image with its last name.
func imageRemoveArgs(image string) []string {
	return []string{"image", "rm", image}
}

// tailWriter keeps the end of what is written to it.
type tailWriter struct {
	b []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if n := len(t.b) - engineStderrKept; n > 0 {
		t.b = t.b[n:]
	}
	return len(p), nil
}

// runEngineCommand runs an engine command whose stdin and stdout are the
// caller's, and adds to its error what the engine said last on its stderr,
// as engineErrorMessage reads it from the commands whose output is read
// whole. A context ended is the error, rather than the kill it made.
func runEngineCommand(ctx context.Context, cmd *exec.Cmd) error {
	stderr := &tailWriter{}
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if ctxDone(ctx) {
		return ctx.Err()
	}
	lines := strings.Split(strings.TrimSpace(string(stderr.b)), "\n")
	if s := strings.TrimSpace(lines[len(lines)-1]); s != "" {
		return fmt.Errorf("%w: %s", err, s)
	}
	return err
}

// saveImage writes on w the archive of an image, with the save command cmd.
// Nothing is kept on the node: the archive goes from the engine to w as it
// is made.
func saveImage(ctx context.Context, cmd *exec.Cmd, w io.Writer) error {
	cmd.Stdout = w
	return runEngineCommand(ctx, cmd)
}

// loadImage loads the archive read from r, with the load command cmd. The
// engine is the one checking the archive: one cut short, as by its source
// failing, is refused, its layers not being the ones its configuration
// names.
func loadImage(ctx context.Context, cmd *exec.Cmd, r io.Reader) error {
	cmd.Stdin = r
	return runEngineCommand(ctx, cmd)
}

// LoadImage loads in the store of the engine the archive of an image read
// from r, as the save of an engine wrote it. It runs as the other commands
// of the container do, so the archive is loaded in the store of the user of
// a rootless container, where its start looks for the image.
func (e *Executor) LoadImage(ctx context.Context, r io.Reader) error {
	a := e.getArgs(imageLoadArgs()...)
	cmd, err := e.command(ctx, a...)
	if err != nil {
		return err
	}
	e.log().Infof("%s %s", e.bin, strings.Join(a, " "))
	return loadImage(ctx, cmd, r)
}

// RemoveImage removes from the store of the engine the name the container
// runs its image by, and the image when it has no other name: a start then
// finds no image, and pulls. It runs as the other commands of the container
// do.
func (e *Executor) RemoveImage(ctx context.Context) error {
	a := e.getArgs(e.args.RemoveImageArgs().Get()...)
	cmd, err := e.command(ctx, a...)
	if err != nil {
		return err
	}
	e.log().Infof("%s %s", e.bin, strings.Join(a, " "))
	return runEngineCommand(ctx, cmd)
}
