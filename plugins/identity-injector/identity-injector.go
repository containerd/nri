package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"strings"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	delegatedidentityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/agent/delegatedidentity/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	// identityKey is the prefix of the key used for identity annotations in the podspec.
	identityKey = "identity.noderesource.dev"
	
	// Default paths for certificate files in the container
	defaultCertFileName      = "svid.pem"
	defaultKeyFileName       = "key.pem"
	defaultBundleFileName    = "bundle.pem"

	// Path in the container where the identity artifacts will be stored
	defaultMountPath     = "/var/run/spiffe/secrets"
)

var (
	log           *logrus.Logger
	verbose       bool
	spireAdminSocket  string
	hostMountPath string
)

type plugin struct {
	stub         stub.Stub
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
	SpiffeId		string `json:"spiffe_id,omitempty"`
}

// containerWatcher tracks a running certificate watcher for a container
type containerWatcher struct {
	cancel context.CancelFunc
	done   chan struct{} 
}

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
		if verbose {
			log.Infof("%s no identity annotations for PID %d", containerName(pod, container), container.Pid) // TODO delete after debugging
		}
		return nil, nil, nil
	}

	// Create host directory for certificates (will be populated in StartContainer)
	hostDir := getHostDir(hostMountPath, pod.GetUid(), container.Name)
	log.Infof("hostMountPath for mounting %s", hostMountPath) // TODO remove after debugging
	log.Infof("hostdir for mounting %s", hostDir)  // TODO remove after debugging
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		log.Errorf("failed to create host directory %s: %v", hostDir, err) // TODO is logging error necessary or will returning the error also log it upstream?
		return nil, nil, fmt.Errorf("failed to create host directory %s: %w", hostDir, err)
	}

	// Add mount for the certificate directory
	mount := &api.Mount{
		Source:      hostDir,
		Destination: config.MountPath,
		Type:        "bind",
		Options:     []string{"ro", "bind"},
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

// Supporting UpdateContainer() is not required because the container pid does not change when a container is updated.
// Supporting StopContainer() addresses both cases - when the container is intentionally stopped under normal operations (graceful exit?) and when the container is stopped if a container crashes
// Remove the watcher and cleanup the certificates for that container on container stop.  
func (p *plugin) StopContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	if verbose {
		dump("RemoveContainer", "pod", pod, "container", container)
	}

	watcherKey := filepath.Join(pod.GetUid(), container.Name)

	// Stop the certificate watcher if it exists
	p.watchersMu.Lock()
	if p.watchers == nil {
		p.watchersMu.Unlock()
		log.Debugf("%s: no watchers map initialized", containerName(pod, container))
	} else if watcher, exists := p.watchers[watcherKey]; exists {
		log.Infof("%s: stopping certificate watcher", containerName(pod, container))
		watcher.cancel()
		delete(p.watchers, watcherKey)
		p.watchersMu.Unlock()

		// Wait for watcher to finish (with timeout)
		select {
		case <-watcher.done:
			log.Infof("%s: certificate watcher stopped gracefully", containerName(pod, container))
		case <-ctx.Done():
			log.Warnf("%s: timeout waiting for certificate watcher to stop", containerName(pod, container))
		}
	} else {
		p.watchersMu.Unlock()
	}

	// Clean up certificate files

	hostDir := getHostDir(hostMountPath, pod.GetUid(), container.Name)
	if err := os.RemoveAll(hostDir); err != nil {
		log.Warnf("%s: failed to clean up certificate directory %s: %v", containerName(pod, container), hostDir, err)
	} else {
		log.Infof("%s: cleaned up certificate directory %s", containerName(pod, container), hostDir)
	}

	return nil, nil
}

func (p *plugin) Shutdown(ctx context.Context) {
	log.Infof("Shutdown called, stopping all certificate watchers")
	
	// Stop all active watchers
	p.watchersMu.Lock()
	if p.watchers == nil {
		p.watchersMu.Unlock()
		log.Warnf("no watchers to stop")
		return
	}
	
	// Cancel all watchers
	for _, watcher := range p.watchers {
		watcher.cancel()
	}

	// Wait for all watchers to finish (with timeout)
	for _, watcher := range p.watchers {
		select {
		case <-watcher.done:
		case <-ctx.Done():
			log.Warnf("timeout waiting for watcher to stop during shutdown")
		}
	}

	// Held the lock for the whole shutdown process.
	p.watchersMu.Unlock()

	log.Infof("all certificate watchers stopped")
}


func (p *plugin) injectIdentity(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	log.Infof("Annotations: %+v", pod.Annotations) // TODO delete after debugging

	// Check container PID
	if container.Pid == 0 {
		return fmt.Errorf("%s container PID not available", containerName(pod, container)) // TODO returning error vs logging error?
	}

	config, err := parseIdentityConfig(container.Name, pod.Annotations)
	if err != nil {
		return err
	}

	if config == nil {
		log.Infof("%s no identity annotations for PID %d", containerName(pod, container), container.Pid) // TODO is this log needed?
		return nil
	}

	if verbose {
		dump(containerName(pod, container), "identity config", config)
	}

	// Get host directory for certificates
	hostDir := getHostDir(hostMountPath, pod.GetUid(), container.Name)

	log.Infof("%s: starting certificate watcher for container PID %d", containerName(pod, container), container.Pid) // TODO delete after debugging

	// Start watching for certificate updates using streaming API
	// This will automatically receive new certificates when they're rotated
	if err := p.startCertificateWatcher(ctx, pod, container, int32(container.Pid), hostDir, config); err != nil {
		return fmt.Errorf("failed to start certificate watcher: %w", err) // TODO log or return error?
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
		log.Debugf("%s: certificate watcher already running", containerName(pod, ctr)) 
		return nil
	}
	p.watchersMu.RUnlock()

	// Create cancellable context for this watcher
	watcherCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// Use WaitGroup to coordinate both goroutines
	var wg sync.WaitGroup
	wg.Add(2) // We have 2 goroutines: SVID watcher and bundle watcher

	// Store watcher info
	p.watchersMu.Lock()
	p.watchers[watcherKey] = &containerWatcher{
		cancel: cancel,
		done:   done,
	}
	p.watchersMu.Unlock()

	// Goroutine to wait for both watchers to finish and then close done channel
	go func() {
		wg.Wait()
		close(done)
		p.watchersMu.Lock()
		delete(p.watchers, watcherKey)
		p.watchersMu.Unlock()
		log.Infof("%s: watcher stopped", containerName(pod, ctr))
	}()

	// Start the SVID watcher goroutine
	go func() {
		defer wg.Done()
		defer cancel() // Cancel context on exit to stop the other goroutine
		defer log.Infof("%s: SVID watcher stopped", containerName(pod, ctr))

		log.Infof("%s: starting certificate stream for PID %d", containerName(pod, ctr), pid) // TODO delete after debugging
		
		req := &delegatedidentityv1.SubscribeToX509SVIDsRequest{
			Pid: pid,
		}

		// TODO remove after debugging
		log.Infof("%s: requesting certificate stream for PID %d using delegated identity API using req %+v", containerName(pod, ctr), pid, req)
		

		// Use SPIRE Delegated Identity API to subscribe to certificate updates
		// This automatically handles certificate rotation - when SPIRE rotates certs,
		// new certificates are pushed to the stream
		stream, err := p.delegatedIdentityClient.SubscribeToX509SVIDs(watcherCtx, req)
		if err != nil {
			// TODO retry logic needed? Yes retry is perhaps needed
			// TODO add a debug log that retry logic is subscribing to svids
			
			// After retries fail, returning here means that the container does not receive the identity artifacts. 
			// If the container does not have artifacts, then the communication using that container will be stopped by security guarantees
			// Therefore the plugin itself does not have to stop the container.
			log.Errorf("%s: failed to subscribe to X509 SVIDs: %v", containerName(pod, ctr), err) // Just log the error. Returning the error not possible because the goroutine here does not return anything
			return
		}

		// Process streaming updates
		// This loop also takes care of retrying to fetch certificates
		for {
			if err := watcherCtx.Err(); err != nil {
				log.Errorf("%s: watcher context cancelled: %v", containerName(pod, ctr), err)
				return
			}

			log.Infof("%s: waiting to receive svid update from stream PID %d", containerName(pod, ctr), pid) // TODO delete after debugging

			resp, err := stream.Recv()
			if err != nil {
				log.Errorf("%s: bundle stream error: %v", containerName(pod, ctr), err)
				return
			}

			// Process the certificate update
			if err := p.processSvidUpdate(containerName(pod, ctr), pid, hostDir, config, resp.X509Svids); err != nil {
				log.Errorf("%s: failed to process certificate update: %v", containerName(pod, ctr), err) // Just log the error. Returning the error not possible because the goroutine here does not return anything

				// this error is a fatal error which should stop further execution/processing
				return
			}

			log.Infof("%s: svid rotated for PID %d using delegated identity API", containerName(pod, ctr), pid) // TODO delete after debugging

		}
	}()

	// Start the bundle watcher goroutine
	go func() {
		defer wg.Done()
		defer cancel() // Cancel context on exit to stop the other goroutine
		defer log.Infof("%s: bundle watcher stopped", containerName(pod, ctr))

		log.Infof("%s: starting bundle stream", containerName(pod, ctr))
		
		req := &delegatedidentityv1.SubscribeToX509BundlesRequest{}

		log.Infof("%s: requesting bundle stream using delegated identity API", containerName(pod, ctr))

		// Use SPIRE Delegated Identity API to subscribe to bundle updates
		// This automatically handles bundle rotation - when SPIRE rotates bundles,
		// new bundles are pushed to the stream
		stream, err := p.delegatedIdentityClient.SubscribeToX509Bundles(watcherCtx, req)
		if err != nil {
			log.Errorf("%s: failed to subscribe to X509 Bundles: %v", containerName(pod, ctr), err)
			return
		}

		// Process streaming updates
		for {
			if err := watcherCtx.Err(); err != nil {
				log.Errorf("%s: watcher context cancelled: %v", containerName(pod, ctr), err)
				return
			}
			
			resp, err := stream.Recv()
			if err != nil {
				log.Errorf("%s: bundle stream error: %v", containerName(pod, ctr), err)
				return
			}

			// Process the bundle update
			if err := p.processBundleUpdate(containerName(pod, ctr), hostDir, config, resp.CaCertificates); err != nil {
				log.Errorf("%s: failed to process bundle update: %v", containerName(pod, ctr), err)
				return
			}

			log.Infof("%s: bundle updated", containerName(pod, ctr))
		}
	}()

	log.Infof("%s: certificate and bundle watchers started for PID %d", containerName(pod, ctr), pid) // TODO delete after debugging
	return nil
}

// processSvidUpdate processes certificate updates from the delegated identity API
func (p *plugin) processSvidUpdate(containerName string, pid int32, hostDir string, config *identityConfig, x509Svids []*delegatedidentityv1.X509SVIDWithKey) error {

	log.Infof("%s: processing svid update for PID %d", containerName, pid) // TODO delete after debugging
	
	if len(x509Svids) == 0 {
		log.Warnf("%s: received empty SVID update for PID %d", containerName, pid)

		// It could take Spire some milliseconds to mint certificates.
		// By not returning error ensures that the for loop in startCertificateWatcher() will also act as retry logic
		return nil
	}

	log.Infof("%s: received %d number of SVIDs in update for PID %d", containerName, len(x509Svids), pid) // TODO deleete after debugging
	
	// TODO delete after debugging
	// Log all SVIDs received -- delete after debugging
	if verbose {
		log.Infof("%s: received certificate update with %s SVID for PID %d:",
			containerName, 
			x509Svids[0].X509Svid.Id, 
			pid,
		)
		
		// TODO delete this log after debugging
		log.Infof("%s: received %d number of certificates update with %s SVID for PID %d:",
			containerName, 
			len(x509Svids[0].X509Svid.CertChain),
			x509Svids[0].X509Svid.Id, 
			pid,
		)
	}

	// TODO implement using hint to select relevant svid in case response has multiple svids
	// Get the default SVID
	svidWithKey := x509Svids[0]

	trustDomain, err := spiffeid.TrustDomainFromString(svidWithKey.X509Svid.Id.TrustDomain)
	if err != nil {
		log.Errorf("%s: failed to parse trust domain: %v", containerName, err)
		return err
	}

	// Compute SpiffeID and compare to the configured SpiffeID from the podspec
	spiffeId, err := spiffeid.FromPath(trustDomain, svidWithKey.X509Svid.Id.Path)
	if err != nil {
		log.Errorf("%s: failed to parse spiffe id from path: %v", containerName, err)
		return err
	}
	log.Infof("%s: parsed spiffe id %s for PID %d", containerName, spiffeId, pid)
	
	if config.SpiffeId != "" && config.SpiffeId != spiffeId.String() {
		return fmt.Errorf("SpiffeId received from Spire Agent does not match the SpiffeId configured in the Podspec")
	}


	// Parse DER encoded certs
	certs := make([]*x509.Certificate, 0, len(svidWithKey.X509Svid.CertChain))
	
	for _, certDER := range svidWithKey.X509Svid.CertChain { // TODO can we not use x509.ParseCertificates() and just parse all of the certs in one go instead of using a loop??
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			log.Errorf("%s: failed to parse certificate: %v", containerName, err)
			return err
		}
		certs = append(certs, cert)
	}

	
	// Parse the DER-encoded private key
	privateKey, err := x509.ParsePKCS8PrivateKey(svidWithKey.X509SvidKey)
	if err != nil {
		log.Errorf("%s: failed to parse private key for %s: %v", containerName, spiffeId, err)
		return err
	}

	// Re-marshal to PKCS#8 DER format 
	// (Even though we already have 'der', re-marshaling is good practice 
	// if you've modified the key or need to ensure standard formatting)
	encodedPrivateKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	// Write updated certificates to host filesystem
	if err := writeX509Content(hostDir, config, certs, encodedPrivateKey); err != nil {
		return fmt.Errorf("failed to write certificates: %w", err)
	}

	log.Infof("%s: certificates updated for PID %d (SPIFFE ID: %s, expires: %d)",
		containerName, pid, svidWithKey.X509Svid.Id.String(), svidWithKey.X509Svid.ExpiresAt)

	return nil
}

// processBundleUpdate processes bundle updates from the delegated identity API
func (p *plugin) processBundleUpdate(containerName string, hostDir string, config *identityConfig, caCertificates map[string][]byte) error {
	if len(caCertificates) == 0 {
		log.Warnf("%s: received empty bundle update", containerName)

		// By not returning error ensures that the for loop in startCertificateWatcher() will also act as retry logic
		return nil
	}

	log.Infof("%s: received bundle update with %d trust domains", containerName, len(caCertificates))

	var bundleSet []*x509.Certificate

	// Among all the containers interacting with a Spire Agent through the NRI Identity Plugin, 
	// if a trust domain is not used by any container/pod interacting with this agent,
	// it means that the agent is misconfigured and that trust domain should be removed from the agent.
	// Parse all CA certificates from all trust domains
	for trustDomain, certDERs := range caCertificates {
		if verbose {
			log.Infof("%s: processing bundle for trust domain %s", containerName, trustDomain)
		}
		
		bundle, err := x509.ParseCertificates(certDERs)
		if err != nil {
			log.Errorf("%s: failed to parse CA certificate for trust domain %s: %v", containerName, trustDomain, err)
			return err
		}

		for _, cert := range bundle {
			bundleSet = append(bundleSet, cert)
		}
	}

	// Write bundle to filesystem
	bundleFile := path.Join(hostDir, config.BundleFileName)
	if err := writeCerts(bundleFile, bundleSet); err != nil {
		return fmt.Errorf("failed to write bundle: %w", err)
	}

	log.Infof("%s: bundle written with %d CA certificates", containerName, len(bundleSet))
	return nil
}

// writeCertificates writes certificates to the host filesystem
// TODO should this function be p.writeX509Content or just writeX509Content? whats the difference?
func writeX509Content(hostDir string, config *identityConfig, certs []*x509.Certificate, privateKey []byte) error { 
	svidFile := path.Join(hostDir, config.CertFileName)
	svidKeyFile := path.Join(hostDir, config.KeyFileName)

	if err := writeCerts(svidFile, certs); err != nil {
		return err
	}

	if err := writePrivateKey(svidKeyFile, privateKey); err != nil {
		return err
	}

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

func parseIdentityConfig(ctr string, annotations map[string]string) (*identityConfig, error) {
	var config identityConfig

	annotation := getAnnotation(annotations, identityKey, ctr)
	if annotation == nil {
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

func getAnnotation(annotations map[string]string, mainKey, ctr string) []byte {
	for _, key := range []string {
		mainKey + "/container." + ctr,
		mainKey + "/pod",
		mainKey,
	} {
		if key == "" || key[0] == '/' {
			continue
		}
		if value, ok := annotations[key]; ok {
			return []byte(value)
		}
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

func ensureUnixPrefix(path string) string {
	if strings.HasPrefix(path, "unix://") {
		return path
	}
	return "unix://" + path
}

func getHostDir(hostMountPath, podUuid, containerName string ) string {
	return filepath.Join(hostMountPath, podUuid, containerName)
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	// TODO uncomment after debugging
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
		opts      []stub.Option
		err       error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.BoolVar(&verbose, "verbose", false, "enable (more) verbose logging")
	flag.StringVar(&spireAdminSocket, "spire-admin-socket", "/run/spire/admin-socket/admin.sock", "SPIRE Delegated Identity API socket path")
	flag.StringVar(&hostMountPath, "host-mount-path", "/var/run/spiffe/secrets/", "Host Volume that will be used for writing identity artifacts of workloads") //TODO Somehow this is not being read from the daemonset yaml
	// Also TODO admin socket is spire but host mount is spiffe... what is the best policy here
	flag.Parse()

	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{
		watchers: make(map[string]*containerWatcher),
	}

	p.delegatedIdentityConn, err = grpc.NewClient(
		ensureUnixPrefix(spireAdminSocket),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		log.Fatalf("failed to create SPIRE Delegated Identity API client: %v", err)
	}
	
	defer p.delegatedIdentityConn.Close()

	p.delegatedIdentityClient = delegatedidentityv1.NewDelegatedIdentityClient(p.delegatedIdentityConn)

	if p.stub, err = stub.New(p, opts...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
