package main

import (
	"context"
	"flag"
	"os"
	"fmt"
	"path/filepath"
	"sync"
	"crypto/x509"
	"encoding/pem"
	"path"

	"sigs.k8s.io/yaml"
	"github.com/sirupsen/logrus"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	delegatedidentityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/agent/delegatedidentity/v1"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	// identityKey is the prefix of the key used for identity annotations.
	identityKey = "identity.noderesource.dev"
	// Default paths for certificate files in the container
	defaultCertFileName      = "svid.pem" //TODO why do we have to put the whole path? can we not do defaultMountPath + "/svid.pem"??
	defaultKeyFileName       = "key.pem"
	defaultBundleFileName    = "bundle.pem"
	defaultMountPath     = "/var/run/spiffe/secrets" // TODO default mountPath is kind of useless here... defaultCertPath has the mountPath init.. ok this is hte path wehere the hostMountPath is mounted to
)

var (
	log           *logrus.Logger
	verbose       bool
	spiffeAgentSocket  string
	spiffeAdminSocket  string
	delegatedIdentitySocket  string
	hostMountPath string
)

type plugin struct {
	stub         stub.Stub
	workloadConn *workloadapi.Client // TODO this is not needed anymore
	x509Source              *workloadapi.X509Source
	delegatedIdentityConn  *grpc.ClientConn
	delegatedIdentityClient delegatedidentityv1.DelegatedIdentityClient
	watchers               map[string]*containerWatcher // key: pod-uid/container-name
	watchersMu             sync.RWMutex
}

// identityConfig represents the configuration for identity injection
type identityConfig struct {
	MountPath  string `json:"mount_path,omitempty"`
	CertFileName  string `json:"cert_file_name,omitempty"`
	KeyFileName    string `json:"key_file_name,omitempty"`
	BundleFileName string `json:"bundle_file_name,omitempty"`
	SpiffeID   string `json:"spiffe_id,omitempty"` // Optional: filter for specific SPIFFE ID // TODO do we want to support users spefiying spiffeID in the podspec? they will need to create a registry for that... but whats the point of having a spiffeId here? all we can do is verify.. that the agent returns this spiffeId.. is this is not returned then we throw error
}

// containerWatcher tracks a running certificate watcher for a container
type containerWatcher struct {
	cancel context.CancelFunc
	done   chan struct{} 
}


// TODO will plugin open multiple connections to the agent, one connection for each pod that starts? since agent is streaming, 
// can we not leverage that streaming - so one connection to get updates for all the containers?

func (p *plugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if verbose {
		dump("CreateContainer", "pod", pod, "container", container)
	}

	 adjust := &api.ContainerAdjustment{}

	config, err := parseIdentityConfig(container.Name, pod.Annotations)
	if err != nil {
		return nil, nil, err
	}

	if config == nil {
		log.Infof("%s no identity annotations for PID %s", containerName(pod, container), container.Pid)
		return nil, nil, nil
	}

	// Create host directory for certificates (will be populated in StartContainer)
	hostDir := filepath.Join(hostMountPath, pod.GetUid(), container.Name)
	log.Infof("hostMountPath for mounting %s", hostMountPath)
	log.Infof("hostdir for mounting %s", hostDir)
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		log.Errorf("failed to create host directory %s: %w", hostDir, err)
		return nil, nil, fmt.Errorf("failed to create host directory %s: %w", hostDir, err)
	}

	// Add mount for the certificate directory
	mount := &api.Mount{
		Source:      hostDir,
		Destination: config.MountPath,
		Type:        "bind",
		Options:     []string{"rw", "rbind"}, // TODO shouldnt it be read only? bb should only read, identity plugin is the one doing the writing
	}
	adjust.AddMount(mount)

	return adjust, nil, nil
}

func (p *plugin) StartContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	if verbose {
		dump("StartContainer", "pod", pod, "container", container)
	}

	log.Infof("Start container entered for container: %s", container.Name) // TODO delete after debugging
	
	return p.injectIdentity(ctx, pod, container)
}

// TODO remove the watcher for that container
func (p *plugin) RemoveContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	//dump("RemoveContainer", "pod", pod, "container", container)
	return nil
}

func (p *plugin) injectIdentity(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Infof("Annotations: %+v", pod.Annotations)
	config, err := parseIdentityConfig(container.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if config == nil {
		log.Infof("%s no identity annotations for PID %s", containerName(pod, container), container.Pid)
		return nil
	}

	if verbose {
		dump(containerName(pod, container), "identity config", config)
	}

	// Get container PID
	containerPID := container.Pid
	if containerPID == 0 {
		return fmt.Errorf("container PID not available")
	}

	// Get host directory for certificates
	// TODO this code is used twice, extract to a method
	hostDir := filepath.Join(hostMountPath, pod.GetUid(), container.Name)

	log.Infof("%s: starting certificate watcher for container PID %d", containerName(pod, container), containerPID)

	// Start watching for certificate updates using streaming API
	// This will automatically receive new certificates when they're rotated
	if err := p.startCertificateWatcher(ctx, pod, container, int32(containerPID), hostDir, config); err != nil {
		return fmt.Errorf("failed to start certificate watcher: %w", err)
	}

	return nil
}

// startCertificateWatcher starts a goroutine that watches for certificate updates
// using the SPIRE Delegated Identity API streaming interface (SubscribeToX509SVIDs).
// This automatically receives new certificates when they're rotated by SPIRE.
func (p *plugin) startCertificateWatcher(ctx context.Context, pod *api.PodSandbox, ctr *api.Container, pid int32, hostDir string, config *identityConfig) error {
	watcherKey := filepath.Join(pod.GetUid(), ctr.Name)	

	// Check if watcher already exists
	p.watchersMu.RLock()
	if _, exists := p.watchers[watcherKey]; exists {
		p.watchersMu.RUnlock()
		log.Infof("%s: certificate watcher already running", containerName(pod, ctr)) // TODO when will this happen? will this happen when container restarts? or when else?
		return nil
	}
	p.watchersMu.RUnlock()

	// Create cancellable context for this watcher
	watcherCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// Store watcher info
	p.watchersMu.Lock()
	p.watchers[watcherKey] = &containerWatcher{
		cancel: cancel,
		done:   done,
	}
	p.watchersMu.Unlock()

	// Start the watcher goroutine
	go func() {
		defer close(done)
		defer func() {
			p.watchersMu.Lock()
			delete(p.watchers, watcherKey)
			p.watchersMu.Unlock()
			log.Infof("%s: certificate watcher stopped", containerName(pod, ctr))
		}()

		log.Infof("%s: starting certificate stream for PID %d using delegated identity API", containerName(pod, ctr), pid)

		// Use SPIRE Delegated Identity API to subscribe to certificate updates
		// This automatically handles certificate rotation - when SPIRE rotates certs,
		// new certificates are pushed to the stream
		req := &delegatedidentityv1.SubscribeToX509SVIDsRequest{
			Pid: pid,
		}

		stream, err := p.delegatedIdentityClient.SubscribeToX509SVIDs(watcherCtx, req)
		if err != nil {
			log.Errorf("%s: failed to subscribe to X509 SVIDs: %v", containerName(pod, ctr), err)
			return
		}

		// Process streaming updates
		for {
			resp, err := stream.Recv() // TODO is this blocking?
			if err != nil {
				if watcherCtx.Err() == nil {
					// Only log error if context wasn't cancelled (i.e., not a graceful shutdown)
					log.Errorf("%s: certificate stream error: %v", containerName(pod, ctr), err)
				}
				return
			}

			// Process the certificate update
			if err := p.processDelegatedIdentityUpdate(pod, ctr, pid, hostDir, config, resp); err != nil {
				log.Errorf("%s: failed to process certificate update: %v", containerName(pod, ctr), err)
			}

			log.Infof("%s: svid rotated for PID %d using delegated identity API", containerName(pod, ctr), pid) // TODO can we log when the cert expires next? problem is there could be multiple certs returned

		}
	}()

	log.Infof("%s: certificate watcher started for PID %d", containerName(pod, ctr), pid)
	return nil
}


// processDelegatedIdentityUpdate processes certificate updates from the delegated identity API
func (p *plugin) processDelegatedIdentityUpdate(pod *api.PodSandbox, ctr *api.Container, pid int32, hostDir string, config *identityConfig, resp *delegatedidentityv1.SubscribeToX509SVIDsResponse) error {
	
	// TODO implement retry logic
	if len(resp.X509Svids) == 0 {
		log.Warnf("%s: received empty SVID update for PID %d", containerName(pod, ctr), pid)
		return nil
	}

	// TODO retry fetching certs because it could be some milliseconds before the certs are minted

	// TODO the only use of the identityconfig is to check if the received svids have the configued spiffeid or not. 
	// is there a better way to do this?
	// it woulkd be useful to support annotating spiffeid in podspec

	// TODO i think we would have also have to add entries to spire-server/agent for each and every pod/container.. otherwise agent wont return svids

	// Log all SVIDs received
	// TODO delete after debugging
	if verbose {
		log.Infof("%s: received certificate update with %s SVID for PID %d:",
			containerName(pod, ctr), 
			resp.X509Svids[0].X509Svid.Id, 
			pid,
		)
	}

	// TODO implement using hint to select relevant svid in case response has multiple svids
	// Get the default SVID
	svidWithKey := resp.X509Svids[0]

	spiffeId, err := spiffeid.FromString(svidWithKey.X509Svid.Id.String())
	if err != nil {
		log.Errorf("%s: failed to parse spiffeId: %v", containerName(pod, ctr), err)
		return err
	}

	// Parse DER encoded certs
	certs := make([]*x509.Certificate, 0, len(svidWithKey.X509Svid.CertChain))
	
	for _, certDER := range svidWithKey.X509Svid.CertChain {
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			log.Errorf("%s: failed to parse certificate: %v", containerName(pod, ctr), err)
			return err
		}
		certs = append(certs, cert)
	}

	
	// Parse the DER-encoded private key
	privateKey, err := x509.ParsePKCS8PrivateKey(svidWithKey.X509SvidKey)
	if err != nil {
		log.Errorf("%s: failed to parse private key for %s: %v", containerName(pod, ctr), spiffeId, err)
		return err
	}

	// Re-marshal to PKCS#8 DER format 
	// (Even though we already have 'der', re-marshaling is good practice 
	// if you've modified the key or need to ensure standard formatting)
	encodedPrivateKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}


	/*
	// Convert SPIRE API response to workloadapi.X509Context format
	x509SVIDs := make([]*workloadapi.X509SVID, 0, len(resp.X509Svids))
	for _, svid := range resp.X509Svids {
		// Parse SPIFFE ID
		spiffeId, err := spiffeid.FromString(svid.X509Svid.Id.String())
		if err != nil {
			log.Warnf("%s: invalid SPIFFE ID %s: %v", containerName(pod, ctr), spiffeId, err)
			continue
		}

		// Parse certificates
		certs := make([]*x509.Certificate, 0, len(svid.Svid.X509Svid))
		for _, certDER := range svid.X509Svid {
			cert, err := x509.ParseCertificate(certDER)
			if err != nil {
				log.Warnf("%s: failed to parse certificate: %v", containerName(pod, ctr), err)
				continue
			}
			certs = append(certs, cert)
		}

		if len(certs) == 0 {
			log.Warnf("%s: no valid certificates for SPIFFE ID %s", containerName(pod, ctr), svid.SpiffeId)
			continue
		}
		
		// TODO is ParsePKCS8PrivateKey() the right method or MarshalPKCS8PrivateKey()?
		// Parse private key
		privateKey, err := x509.ParsePKCS8PrivateKey(svid.X509SvidKey)
		if err != nil {
			log.Warnf("%s: failed to parse private key for %s: %v", containerName(pod, ctr), svid.SpiffeId, err)
			continue
		}

		x509SVIDs = append(x509SVIDs, &workloadapi.X509SVID{
			ID:           id,
			Certificates: certs,
			PrivateKey:   privateKey,
		})
	} */

	// TODO implement a retry logic when we dont get any certs
	// if len(x509SVIDs) == 0 {
	// 	return fmt.Errorf("no valid SVIDs in update")
	// }

	/* None of this comment block is needed
	// Convert bundles to workloadapi format
	bundles := make(map[string][]*x509.Certificate)
	for trustDomain, bundle := range resp.FederatedBundles {
		certs := make([]*x509.Certificate, 0, len(bundle.X509Authorities))
		for _, certDER := range bundle.X509Authorities {
			cert, err := x509.ParseCertificate(certDER)
			if err != nil {
				log.Warnf("%s: failed to parse bundle certificate for %s: %v", containerName(pod, ctr), trustDomain, err)
				continue
			}
			certs = append(certs, cert)
		}
		if len(certs) > 0 {
			bundles[trustDomain] = certs
		}
	}

	bundleSet, err := workloadapi.NewX509BundleSet(bundles)
	if err != nil {
		return fmt.Errorf("failed to create bundle set: %w", err)
	}

	// Filter by SPIFFE ID if specified
	var filteredContext *workloadapi.X509Context
	if config.SpiffeID != "" {
		for _, svid := range x509SVIDs {
			if svid.ID.String() == config.SpiffeID {
				filteredContext = &workloadapi.X509Context{
					SVIDs:   []*workloadapi.X509SVID{svid},
					Bundles: bundleSet,
				}
				break
			}
		}
		if filteredContext == nil {
			log.Warnf("%s: requested SPIFFE ID %s not found in update for PID %d",
				containerName(pod, ctr), config.SpiffeID, pid)
			return nil
		}
	} else {
		filteredContext = &workloadapi.X509Context{
			SVIDs:   x509SVIDs,
			Bundles: bundleSet,
		}
	} */

	// Write updated certificates to host filesystem
	if err := writeX509Content(hostDir, config, certs, encodedPrivateKey); err != nil {
		return fmt.Errorf("failed to write certificates: %w", err)
	}

	log.Infof("%s: updated certificates for PID %d (SPIFFE ID: %s, expires: %s)",
		containerName(pod, ctr), pid, svidWithKey.X509Svid.Id.String(), svidWithKey.X509Svid.ExpiresAt)

	

	return nil
}

// writeCertificates writes certificates to the host filesystem
// Made into a standalone function so it can be used by both initial injection and updates
func writeX509Content(hostDir string, config *identityConfig, certs []*x509.Certificate, privateKey []byte) error {
	svidFile := path.Join(hostDir, config.CertFileName)
	svidKeyFile := path.Join(hostDir, config.KeyFileName)

	if err := writeCerts(svidFile, certs); err != nil {
		return err
	}

	if err := writePrivateKey(svidKeyFile, privateKey); err != nil {
		return err
	}

	/* TODO pending to implement writing bundles
	// Write trust bundle
	bundlePath := filepath.Join(hostDir, "bundle.pem")
	bundlePEM := encodeTrustBundle(x509Context.Bundles)
	if err := os.WriteFile(bundlePath, bundlePEM, 0644); err != nil {
		return fmt.Errorf("failed to write trust bundle: %w", err)
	}
	*/

	return nil
}

func writeCerts(file string, certs []*x509.Certificate) error {
	var pemData []byte
	for _, cert := range certs {

		// TODO do we need to implement not writing expired certs?
		b := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}
		pemData = append(pemData, pem.EncodeToMemory(b)...)
	}
	return os.WriteFile(file, pemData, 0644)
}

func writePrivateKey(file string, privateKey []byte) error {
	b := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKey,
	}

	return os.WriteFile(file, pem.EncodeToMemory(b), 0600)
}

/* TODO pending to write and handle bundles
func encodeTrustBundle(bundles *workloadapi.X509BundleSet) []byte {
	var pemData []byte
	for _, bundle := range bundles.Bundles() {
		for _, cert := range bundle.X509Authorities() {
			pemData = append(pemData, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cert.Raw,
			})...)
		}
	}
	return pemData
}
*/


// TODO 
func (p *plugin) Shutdown(ctx context.Context) {}

type identityInjectorSvidWatcher struct {} // TODO also probably not needed

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

// TODO is this really needed?  YES this is needed. We need it with annotations and the k8s KEP too pod certificates
func parseIdentityConfig(ctr string, annotations map[string]string) (*identityConfig, error) {
	var config identityConfig

	annotation := getAnnotation(annotations, identityKey, ctr)
	if annotation == nil {
		log.Infof("No annotations")
		return nil, nil
	}

	if err := yaml.Unmarshal(annotation, &config); err != nil {
		return nil, fmt.Errorf("invalid identity annotation %q: %w", string(annotation), err)
	}

	// Set default paths if not specified
	if config.MountPath == "" {
		config.MountPath = defaultMountPath
	}
	if config.CertFileName == "" {
		config.CertFileName = defaultCertFileName
	}
	if config.KeyFileName == "" {
		config.KeyFileName = defaultKeyFileName
	}
	if config.BundleFileName == "" {
		config.BundleFileName = defaultBundleFileName
	}

	return &config, nil
}

func getAnnotation(annotations map[string]string, key, ctr string) []byte {
	// TODO we need to support annotion without container name, which would mean add the same config to all containers, see device injector
	if value, ok := annotations[key + "/container." + ctr]; ok {
		return []byte(value)
	}

	return nil
}

// Construct a container name for log messages.
func containerName(pod *api.PodSandbox, container *api.Container) string {
	if pod != nil {
		return pod.Name + "/" + container.Name
	}
	return container.Name
}


// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	// TODO uncomment
	/* var (
		prefix string
		idx    int
	)

	if len(args)&0x1 == 1 {
		prefix = args[0].(string)
		idx++
	}

	for ; idx < len(args)-1; idx += 2 {
		tag, obj := args[idx], args[idx+1]
		msg, err := yaml.Marshal(obj)
		if err != nil {
			log.Infof("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			log.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s:    %s", prefix, line)
			}
		} else {
			log.Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("  %s", line)
			}
		}
	} */
}


func main() {
	var (
		pluginIdx string
		events     string
		opts      []stub.Option
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.StringVar(&events, "events", "", "comma-separated list of events to subscribe for")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.StringVar(&spiffeAgentSocket, "spiffe-agent-socket", "unix:///run/spire/sockets/agent.sock", "SPIFFE Workload API socket path")
	flag.StringVar(&spiffeAdminSocket, "spiffe-admin-socket", "unix:///run/spire/admin-socket/admin.sock", "SPIFFE Delegated Identity API socket path")
	flag.StringVar(&hostMountPath, "host-mount-path", "/var/run/spiffe/secrets/", "Host Volume that will be used for writing identity artifacts of workloads")
	// TODO flag.StringVar(&hostMountPath, "host-mount-path", defaultHostMountPath, "host path for mounting certificates")
	flag.Parse()

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	} /* TODO delete this else at the end */ else { 
		opts = append(opts, stub.WithPluginIdx("11"))
	}

	


	p := &plugin{
		watchers: make(map[string]*containerWatcher),
	}

	// Initialize SPIFFE workload API client
	ctx := context.Background()


	/* TODO The old way of doing things not using source, delete when  source is working
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
	}() */

	// 1. Establish a connection to the local Workload API to get this delegate's identity
	// This replaces manual certificate management.
	// Automatically rotates certificates
	
	p.x509Source, err = workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(spiffeAgentSocket)))
	if err != nil {
		log.Fatalf("failed to create X509 source: %v", err)
	}
	defer p.x509Source.Close()

	log.Infof("Connected to SPIFFE Workload API at %s", spiffeAgentSocket)

	var initialSvid *x509svid.SVID
	// Get initial SVID to log plugin identity
	initialSvid, err = p.x509Source.GetX509SVID()
	if err != nil {
		log.Fatalf("failed to get plugin SVID: %v", err)
	}

	if verbose /* TODO remove this true at the end */ || true {
		log.Infof("Plugin identity details:")
		log.Infof("Trust Domain: %s", initialSvid.ID.TrustDomain().String())
		log.Infof("Certificate expires: %s", initialSvid.Certificates[0].NotAfter)
	}

	// Start a goroutine to monitor SVID rotation using the X509Source watcher
	go func() {
		for {
			// WaitUntilUpdated blocks until the SVID is rotated
			err := p.x509Source.WaitUntilUpdated(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled, shutting down
					log.Fatalf("Context cancelled: %v", err)
					// TODO what else can we do here?
				}
				log.Warnf("Error waiting for SVID update: %v", err)
				continue
			}
			
			// SVID has been rotated, get the new one and log it
			newSVID, err := p.x509Source.GetX509SVID()
			if err != nil {
				log.Warnf("Failed to get SVID after rotation: %v", err)
				continue
			}
			
			log.Infof("Plugin SVID rotated: %s (new certificate expires: %s)",
				newSVID.ID.String(), newSVID.Certificates[0].NotAfter)
		}
	}()






	// 2. Configure mTLS using the SVIDs provided by SPIRE
	// This ensures the Agent knows exactly who the delegate is.
	//tlsConf := tlsconfig.MTLSClientConfig(p.x509Source, p.x509Source, tlsconfig.AuthorizeAny())
	// TODO check nuanaces. TLS not needed because all communication is happening on the same host, but check nuances

	p.delegatedIdentityConn, err = grpc.NewClient(
		spiffeAdminSocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		log.Fatalf("failed to create SPIRE Delegated Identity API client: %v", err)
	}
	
	defer p.delegatedIdentityConn.Close()

	p.delegatedIdentityClient = delegatedidentityv1.NewDelegatedIdentityClient(p.delegatedIdentityConn)
	// TODO the log message below is wrong, the live above doesnt really connect to the delegated api. 
	// We should connect to the delegated api here and if error return the error instead of starting the plugin
	log.Infof("Connected to SPIRE Delegated Identity API at %s", delegatedIdentitySocket)


	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	log.Infof("Plugin stub created") // TODO delete after debugging

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
