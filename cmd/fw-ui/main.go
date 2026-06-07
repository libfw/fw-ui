// Command fw-ui serves a real-time web UI for the socket_vmnet firewall by
// connecting to its --control-socket and exposing a Huma REST + SSE API plus an
// embedded Svelte/DaisyUI frontend.
package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libfw/fw-ui/internal/fwui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	var control, listen string
	var interval time.Duration
	var samples, events int

	root := &cobra.Command{
		Use:           "fw-ui",
		Short:         "Real-time web UI for the socket_vmnet firewall",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			c := fwui.New(control)
			defer c.Close()
			p := fwui.NewPoller(c, interval, samples, events)
			go p.Run(ctx)

			sub, err := fs.Sub(staticFS, "static")
			if err != nil {
				return err
			}
			srv := fwui.NewServer(p, c, sub)
			httpSrv := &http.Server{Addr: listen, Handler: srv.Handler()}
			go func() {
				<-ctx.Done()
				sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(sctx)
			}()

			log.Printf("fw-ui: control=%s listen=http://%s poll=%s", control, listen, interval)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}

	f := root.Flags()
	f.StringVar(&control, "control-socket", "", "path to the daemon's --control-socket (required)")
	f.StringVar(&listen, "listen", "127.0.0.1:8849", "HTTP listen address")
	f.DurationVar(&interval, "poll-interval", time.Second, "counter poll interval")
	f.IntVar(&samples, "samples", 300, "number of rate samples retained")
	f.IntVar(&events, "events", 1000, "number of recent events retained")
	_ = root.MarkFlagRequired("control-socket")

	if err := root.Execute(); err != nil {
		log.Printf("fw-ui: %v", err)
		os.Exit(1)
	}
}
