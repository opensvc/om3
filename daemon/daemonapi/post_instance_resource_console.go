package daemonapi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) PostInstanceResourceConsole(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceResourceConsoleParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.localInstanceResourceConsole(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceResourceConsole(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func scanForConsoleUrl(r io.Reader, timeout time.Duration) (string, error) {
	scanner := bufio.NewScanner(r)
	doneC := make(chan string)
	pattern := "public session: "
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, pattern) {
				doneC <- line[len(pattern):]
				return
			}
		}
		doneC <- ""
	}()

	select {
	case <-timer.C:
		return "", fmt.Errorf("timeout waiting for console url")
	case url := <-doneC:
		if url == "" {
			return "", fmt.Errorf("console url not found")
		} else {
			return url, nil
		}
	}
}

func (a *DaemonAPI) localInstanceResourceConsole(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceResourceConsoleParams) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}

	node, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	config := node.MergedConfig()

	consoleServer := config.GetString(key.New("console", "server"))
	if consoleServer == "" {
		return JSONProblemf(ctx, http.StatusServiceUnavailable, "Service Unavailable", "console.server is not set")

	}

	maxGreetTimeout, err := config.GetDurationStrict(key.New("console", "max_greet_timeout"))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Capping GreetTimeout error", "%s", err)
	}

	maxSeats, err := config.GetIntStrict(key.New("console", "max_seats"))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Capping Seats error", "%s", err)
	}

	enterArgs := fmt.Sprintf("%s enter", path)
	if params.Rid != nil {
		enterArgs += fmt.Sprintf(" --rid %s", *params.Rid)
	}

	args := []string{
		"-command", os.Args[0],
		"-args", enterArgs,
		"-public", "-tty-proxy", consoleServer,
		"-no-wait",
		"-headless",
		"-listen", ":0",
	}
	if params.Seats != nil {
		if *params.Seats == 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad Request", "Ulimited (0) Seats is not allowed")
		} else if *params.Seats > maxSeats {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad Request", "Seats %d is too high: max %d", *params.Seats, maxSeats)
		} else {
			args = append(args, "-seats", fmt.Sprintf("%d", *params.Seats))
		}
	} else {
		args = append(args, "-seats", "1")
	}
	if params.GreetTimeout != nil {
		if d, err := time.ParseDuration(*params.GreetTimeout); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad Request", "GreetTimeout %s is not a valid duration: %s", *params.GreetTimeout, err)
		} else if d == 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad Request", "Unlimited GreetTimeout (0) is not allowed")
		} else if d > *maxGreetTimeout {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad Request", "GreetTimeout %s is too long: max %s", *params.GreetTimeout, *maxGreetTimeout)
		} else {
			args = append(args, "-greet-timeout", fmt.Sprintf("%s", *params.GreetTimeout))
		}
	} else {
		args = append(args, "-greet-timeout", "5s")
	}
	if config.GetBool(key.New("console", "insecure")) {
		args = append(args, "-k")
	}
	r, w, err := os.Pipe()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "new command stdout pipe", "%s", err)
	}
	cmd := exec.Command("tty-share", args...)
	cmd.Stdin = nil
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	err = cmd.Start()
	if err != nil {
		r.Close()
		w.Close()
		return JSONProblemf(ctx, http.StatusInternalServerError, "start console share", "%s", err)
	}
	go func() {
		cmd.Wait()
		r.Close()
		w.Close()
	}()
	url, err := scanForConsoleUrl(r, 2*time.Second)
	if err != nil {
		cmd.Process.Kill()
		return JSONProblemf(ctx, http.StatusInternalServerError, "scan for console url", "%s", err)
	}
	ctx.Response().Header().Set("Location", url)
	return ctx.NoContent(http.StatusCreated)
}
