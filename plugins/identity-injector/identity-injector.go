package main

import (
	"context"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/containerd/nri/pkg/stub"
)

const (
	// identityKey is the prefix of the key used for identity annotations.
	// identityKey = "identity.spiffe.io"
	// Default paths for certificate files in the container
	// defaultCertPath      = "/var/run/secrets/spiffe/svid.pem"
	// defaultKeyPath       = "/var/run/secrets/spiffe/key.pem"
	// defaultBundlePath    = "/var/run/secrets/spiffe/bundle.pem"
	// defaultMountPath     = "/var/run/secrets/spiffe"
	defaultHostMountPath = "/var/run/spiffe"
)

var (
	log           *logrus.Logger
	verbose       bool
	spiffeSocket  string
	hostMountPath string
)

type plugin struct {
	stub         stub.Stub
	workloadConn *workloadapi.Client
}

type identityInjectorSvidWatcher struct {}

func (w *identityInjectorSvidWatcher) OnX509ContextUpdate(c *workloadapi.X509Context) {
	if len(c.SVIDs) == 0 {
		log.Fatalf("no SVIDs available for plugin (check SPIRE registration)")
		return
	}

	svid := c.DefaultSVID()

	if svid == nil {
		log.Errorf("Default SVID is nil")
		return
	}
	
	log.Printf("SVID Updated!")
	log.Printf("SPIFFE ID: %s", svid.ID)
	log.Printf("Expires:   %v", svid.Certificates[0].NotAfter)
	
	// bundle := c.Bundles
}

func (w *identityInjectorSvidWatcher) OnX509ContextWatchError(err error) {
	log.Printf("SPIRE Watch Error: %v", err)
}


func main() {
	var (
		pluginIdx string
		opts      []stub.Option
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.StringVar(&spiffeSocket, "spiffe-socket", "unix:///run/spire/sockets/agent.sock", "SPIFFE Workload API socket path")
	flag.StringVar(&hostMountPath, "host-mount-path", defaultHostMountPath, "host path for mounting certificates")
	flag.Parse()

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{}

	// Initialize SPIFFE workload API client
	ctx := context.Background()
	p.workloadConn, err = workloadapi.New(ctx, workloadapi.WithAddr(spiffeSocket))
	if err != nil {
		log.Fatalf("failed to create SPIFFE workload API client: %v", err)
	}
	defer p.workloadConn.Close()

	log.Infof("Connected to SPIFFE Workload API at %s", spiffeSocket)

	go func() {
		err = p.workloadConn.WatchX509Context(ctx, &identityInjectorSvidWatcher{})
		if err != nil {
			log.Fatalf("failed to fetch plugin identity from Workload API: %v", err)
		}
	}()


	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
