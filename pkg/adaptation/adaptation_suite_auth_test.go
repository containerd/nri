/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package adaptation_test

import (
	"context"
	"errors"
	"fmt"
	stdnet "net"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sync"
	"testing"
	"time"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/auth"
	"github.com/containerd/nri/pkg/auth/ecdh"
	"github.com/containerd/nri/pkg/net/multiplex"
	"github.com/containerd/nri/pkg/stub"
	"github.com/containerd/ttrpc"
	"github.com/stretchr/testify/require"
)

func TestPluginAuthentication(t *testing.T) {
	type Keys struct {
		private []byte
		public  []byte
	}

	var (
		key0 = &Keys{
			private: []byte("TkCE8UGWdLKjXzrUzc6AI6pFuma39+wL4d1P/gyqI0A="),
			public:  []byte("OlqYOu+/5pP9S+6XpeiC3ryr/iKEkZiTdo9VmMohbFY="),
		}
		key1 = &Keys{
			private: []byte("26zy0XoZOAGW5y1HwzR3dxbCr/K2pZaZvQO+8VQUgWs="),
			public:  []byte("PG+8RDcmJ+bznMdR0RpDjJHju+DTFnmkiv9e71yndnk="),
		}

		runtimeOptions = []nri.Option{
			nri.WithAuthConfig(&auth.Config{
				Roles: map[string]*auth.Role{
					"role0": {
						Keys: []string{
							string(key0.public),
						},
					},
					"role1": {
						Keys: []string{
							string(key1.public),
						},
					},
				},
			}),
		}
	)

	s := &Suite{}

	t.Run("should succeed with the correct key", func(t *testing.T) {
		s.Prepare(
			t,
			&mockRuntime{options: runtimeOptions},
			&mockPlugin{
				idx:  "00",
				name: "foo",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key0.private),
							slices.Clone(key0.public),
						),
					),
				},
			},
			&mockPlugin{
				idx:  "10",
				name: "bar",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key1.private),
							slices.Clone(key1.public),
						),
					),
				},
			},
		)
		s.Startup()

		s.WaitForPluginsToSync(s.plugin("00-foo"), s.plugin("10-bar"))
		require.Equal(t, s.plugin("00-foo").stub.GetRole(), "role0")
		require.Equal(t, s.plugin("10-bar").stub.GetRole(), "role1")
	})

	t.Run("should fail with incorrect keys", func(t *testing.T) {
		s.Prepare(
			t,
			&mockRuntime{options: runtimeOptions},
			&mockPlugin{
				idx:  "00",
				name: "foo",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key0.private),
							slices.Clone(key1.public),
						),
					),
				},
			},
			&mockPlugin{
				idx:  "10",
				name: "bar",
				options: []stub.Option{
					stub.WithAuthentication(
						nri.DefaultAuthentication,
						auth.NewMemKeyFetcher(
							slices.Clone(key1.private),
							slices.Clone(key0.public),
						),
					),
				},
			},
		)
		s.StartRuntime()
		require.Error(t, s.StartPlugin(s.plugins[0]))
		require.Error(t, s.StartPlugin(s.plugins[1]))
	})
}

// TestAuthenticationWithKeyDirFromEnvironment verifies that a plugin
// picks up its authentication keys from the directory named by the
// NRI_PLUGIN_AUTH_KEYDIR environment variable.
//
// This is how a plugin deployed as a DaemonSet gets its keys, without
// having to know anything about authentication itself: the stub reads
// the private and public key from that directory and authenticates
// with them, as long as the plugin was not set up with keys of its own.
func TestAuthenticationWithKeyDirFromEnvironment(t *testing.T) {
	private, public, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	keyDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "private"), private.Encode(), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "public"), public.Encode(), 0o644))

	t.Setenv(api.PluginAuthKeyDirEnvVar, keyDir)

	var (
		s = &Suite{}
		// Note: no stub.WithAuthentication() for this plugin.
		plugin = &mockPlugin{idx: "00", name: "foo"}
	)

	s.Prepare(t,
		&mockRuntime{
			options: []nri.Option{
				nri.WithAuthConfig(&auth.Config{
					Roles: map[string]*auth.Role{
						"privileged": {
							Keys: []string{string(public.Encode())},
							Tags: map[string]string{"cap": "all"},
						},
					},
				}),
			},
		},
		plugin,
	)

	s.Startup()
	s.WaitForPluginsToSync(s.plugin("00-foo"))

	require.Equal(t, "privileged", plugin.stub.GetRole(),
		"plugin did not authenticate with the keys from the environment")
	require.Equal(t, map[string]string{"cap": "all"}, plugin.stub.GetTags())
}

// TestAuthenticationBypassAttempts verifies that a plugin cannot get
// the role configured for a public key without proving possession of
// the matching private key.
//
// Public keys are not secrets. They are part of the runtime
// configuration. The plugins here know the public key of a privileged
// role and nothing else. There are two ways for them to try: start
// authenticating and never answer the challenge, or register first and
// start authenticating afterwards. The runtime refuses both, so neither
// plugin ever carries a role into validation.
func TestAuthenticationBypassAttempts(t *testing.T) {
	// The private key is generated only to get a matching public key
	// for the runtime configuration. These plugins never use it.
	_, public, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	authConfig := &auth.Config{
		Roles: map[string]*auth.Role{
			"privileged": {
				Keys: []string{string(public.Encode())},
				Tags: map[string]string{"cap": "all"},
			},
		},
	}

	t.Run("challenge response skipped, then registration", func(t *testing.T) {
		s, validateC := startAuthSuite(t, authConfig)
		rogue := &roguePlugin{
			idx:       "10",
			name:      "rogue",
			publicKey: public.Encode(),
			mask:      api.MustParseEventMask("RunPodSandbox,CreateContainer"),
		}

		require.NoError(t, rogue.Connect(s.Dir()))
		t.Cleanup(rogue.Stop)

		challenge, err := rogue.RequestChallenge(ecdh.Name, rogue.publicKey)
		require.NoError(t, err)

		// Sanity check for the test setup itself: the runtime must have
		// recognized the public key and issued a challenge for it.
		require.NotEmpty(t, challenge.GetChallenge(),
			"runtime did not challenge the plugin")

		// A plugin would prove possession of the private key here. This
		// one cannot, it only knows the public key, so it goes straight
		// to registration.
		require.Error(t, rogue.Register(),
			"plugin registered without finishing authentication")

		require.NotContains(t, validatedPlugins(t, s, validateC, "0"), rogue.fullName(),
			"plugin which abandoned authentication took part in validation")
	})

	t.Run("registration first, then authentication", func(t *testing.T) {
		s, validateC := startAuthSuite(t, authConfig)
		late := &roguePlugin{
			idx:       "20",
			name:      "late",
			publicKey: public.Encode(),
			mask:      api.MustParseEventMask("RunPodSandbox,CreateContainer"),
		}

		require.NoError(t, late.Connect(s.Dir()))
		t.Cleanup(late.Stop)
		require.NoError(t, late.Register())

		// Wait until the runtime has synchronized and activated the plugin.
		require.NoError(t, late.WaitForSync(startupTimeout))
		s.runtime.runtime.BlockPluginSync().Unblock()

		// The plugin is connected but never authenticated, so it takes
		// part in validation without a role.
		instance := validatedPlugins(t, s, validateC, "1")[late.fullName()]
		require.NotNil(t, instance, "plugin missing from validation request")
		require.Equal(t, "", instance.GetRole(),
			"plugin which never authenticated was granted a role")
		require.Empty(t, instance.GetTags(),
			"plugin which never authenticated was granted role tags")

		// Authenticating now is refused, so the role of the key cannot
		// be picked up after registration.
		_, err := late.RequestChallenge(ecdh.Name, late.publicKey)
		require.Error(t, err, "plugin authenticated after registration")

		// The refusal costs the plugin its connection, so it drops out
		// of validation altogether.
		require.NotContains(t, validatedPlugins(t, s, validateC, "2"), late.fullName(),
			"plugin which authenticated late stayed connected")
	})
}

// TestReRegistrationRefused verifies that a plugin cannot register a
// second time, and in particular cannot change the identity it is
// registered with.
//
// The runtime knows a plugin by the index and name it registers with.
// Validation attributes adjustments to that identity and the default
// validator checks required plugins by it, so a plugin able to rename
// itself after registration could take the place of another plugin as
// far as validation is concerned.
func TestReRegistrationRefused(t *testing.T) {
	private, public, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	authConfig := &auth.Config{
		Roles: map[string]*auth.Role{
			"privileged": {
				Keys: []string{string(public.Encode())},
				Tags: map[string]string{"cap": "all"},
			},
		},
	}

	// check registers the plugin, has it try to register again under
	// another identity, then verifies that validation still sees it
	// under its original identity and role.
	check := func(t *testing.T, p *roguePlugin, role string) {
		t.Helper()

		s, validateC := startAuthSuite(t, authConfig)

		require.NoError(t, p.Connect(s.Dir()))
		t.Cleanup(p.Stop)

		if role != "" {
			result, err := p.Authenticate(ecdh.Name, private.Encode(), public.Encode())
			require.NoError(t, err)
			require.Equal(t, role, result.GetRole(), "authenticated to unexpected role")
		}

		require.NoError(t, p.Register())
		require.NoError(t, p.WaitForSync(startupTimeout))
		s.runtime.runtime.BlockPluginSync().Unblock()

		// The refusal must be the one for registering twice. This is
		// the single refusal whose error reaches the plugin reliably,
		// because it is the one which reports no status, so nothing
		// tears the connection down before the response is flushed.
		require.ErrorContains(t, p.RegisterAs("90", "impostor"), "already registered",
			"plugin registered a second time, or was refused for another reason")

		plugins := validatedPlugins(t, s, validateC, "0")

		require.NotContains(t, plugins, "90-impostor",
			"plugin registered under an identity it was refused")

		instance := plugins[p.fullName()]
		require.NotNil(t, instance, "plugin lost its identity to a refused registration")
		require.Equal(t, role, instance.GetRole(), "plugin lost its role")
	}

	t.Run("unauthenticated plugin", func(t *testing.T) {
		check(t, &roguePlugin{
			idx:  "10",
			name: "plain",
			mask: api.MustParseEventMask("RunPodSandbox,CreateContainer"),
		}, "")
	})

	t.Run("authenticated plugin", func(t *testing.T) {
		check(t, &roguePlugin{
			idx:       "20",
			name:      "authenticated",
			publicKey: public.Encode(),
			mask:      api.MustParseEventMask("RunPodSandbox,CreateContainer"),
		}, "privileged")
	})
}

// TestRepeatedRegistrationIsRefusedPromptly verifies that a plugin
// registering over and over gets a prompt refusal every time.
//
// The runtime reports the outcome of a registration request to the
// goroutine waiting for the plugin to register. That goroutine waits
// for one outcome and then moves on, so for any later request there is
// nobody left to receive one. Such a request must be refused without
// waiting for a receiver which is no longer there, or its handler stays
// blocked for the lifetime of the runtime, holding onto the goroutine
// ttrpc gave it. A plugin repeating the request would leak one of those
// per attempt. The request context does not help: a blocked send to a
// channel ignores it, so the deadline releases the plugin and leaves
// the runtime side parked.
//
// Authentication has no part in this. The test runs without an
// authentication configuration to keep it that way.
func TestRepeatedRegistrationIsRefusedPromptly(t *testing.T) {
	const (
		// Long enough that no prompt refusal reaches it, short enough
		// that a blocked handler does not stall the test for long.
		requestTimeout = 2 * time.Second
	)

	var (
		s     = &Suite{}
		rogue = &roguePlugin{
			idx:  "10",
			name: "rogue",
			mask: api.MustParseEventMask("RunPodSandbox"),
		}
	)

	s.Prepare(t, &mockRuntime{})
	s.Startup()

	require.NoError(t, rogue.Connect(s.Dir()))
	t.Cleanup(rogue.Stop)

	require.NoError(t, rogue.Register())
	require.NoError(t, rogue.WaitForSync(startupTimeout))

	// Three attempts, not one: an outcome reported for a refused
	// request fits in the buffer of the channel once, so the attempt
	// which fills it is refused promptly and the ones after it are the
	// ones left waiting.
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		started := time.Now()
		err := rogue.RegisterAsWithContext(ctx, "90", "impostor")
		elapsed := time.Since(started)
		cancel()

		// Promptness first: it is the failure this test exists for, and
		// its message says more than a mismatched error would. The
		// error itself must then be the refusal for registering twice,
		// not a deadline the runtime let the plugin run into.
		require.Less(t, elapsed, requestTimeout,
			"registration attempt %d blocked until the request timed out", attempt)
		require.ErrorContains(t, err, "already registered",
			"registration attempt %d was not refused as a repeat", attempt)
	}
}

// TestRegistrationWithIncompleteAuthentication verifies that a plugin
// which starts authentication but never completes it is not allowed to
// register.
//
// Such a plugin gets no role, so it cannot gain privileges it failed to
// prove. It should not get connected either. It asked for a challenge,
// so it has keys and intends to use them. Failing to answer the
// challenge is a protocol violation, not a reason to fall back to
// connecting as an unauthenticated plugin.
//
// A plugin which never starts authentication is a different case. It is
// allowed to register, it simply gets no role. The second subtest pins
// that down, so that rejecting an incomplete handshake does not turn
// into rejecting plugins which never attempt one.
func TestRegistrationWithIncompleteAuthentication(t *testing.T) {
	_, public, err := ecdh.GenerateKeyPair()
	require.NoError(t, err)

	authConfig := &auth.Config{
		Roles: map[string]*auth.Role{
			"privileged": {
				Keys: []string{string(public.Encode())},
				Tags: map[string]string{"cap": "all"},
			},
		},
	}

	setup := func(t *testing.T) *Suite {
		s := &Suite{}
		s.Prepare(t,
			&mockRuntime{
				options: []nri.Option{
					nri.WithAuthConfig(authConfig),
				},
			},
		)
		s.Startup()

		return s
	}

	t.Run("plugin which skips the challenge response", func(t *testing.T) {
		s := setup(t)
		rogue := &roguePlugin{
			idx:       "10",
			name:      "rogue",
			publicKey: public.Encode(),
			mask:      api.MustParseEventMask("RunPodSandbox"),
		}

		require.NoError(t, rogue.Connect(s.Dir()))
		t.Cleanup(rogue.Stop)

		challenge, err := rogue.RequestChallenge(ecdh.Name, rogue.publicKey)
		require.NoError(t, err)
		require.NotEmpty(t, challenge.GetChallenge(),
			"runtime did not challenge the plugin")

		// A plugin would respond to the challenge here. This one does
		// not, it goes straight to registration instead.

		// The runtime can reject the registration request itself, or
		// drop the connection, so the error of the request is only
		// logged. What must hold either way is that the plugin does not
		// end up configured and synchronized as a connected plugin.
		t.Logf("registration of unauthenticated plugin returned: %v", rogue.Register())

		require.Error(t, rogue.WaitForSync(startupTimeout),
			"plugin was connected without completing the authentication it started")
	})

	t.Run("plugin which never starts authentication", func(t *testing.T) {
		s := setup(t)
		plain := &roguePlugin{
			idx:  "20",
			name: "plain",
			mask: api.MustParseEventMask("RunPodSandbox"),
		}

		require.NoError(t, plain.Connect(s.Dir()))
		t.Cleanup(plain.Stop)

		require.NoError(t, plain.Register(),
			"plugin which never attempted authentication was rejected")
		require.NoError(t, plain.WaitForSync(startupTimeout),
			"plugin which never attempted authentication was not connected")
	})
}

// TestAuthenticationAttemptWithoutRuntimeAuthConfig verifies that a
// plugin trying to authenticate with a runtime which has no
// authentication configuration gets rejected, instead of crashing the
// runtime.
//
// The runtime registers the authentication service for every connected
// plugin, whether it has an authentication configuration or not. If it
// has none, r.auth is nil and the challenge handler dereferences it.
// ttrpc does not recover panics in its per-request goroutines, so in a
// real runtime such a panic takes down the whole process. Any plugin
// able to connect to the socket can trigger it, with a made-up key.
//
// This test installs a recovering ttrpc interceptor in the runtime, so
// that the panic fails this test instead of killing the test binary.
// Nothing recovers the panic in a real runtime.
func TestAuthenticationAttemptWithoutRuntimeAuthConfig(t *testing.T) {
	var (
		s        = &Suite{}
		recorder = &panicRecorder{}
		rogue    = &roguePlugin{
			idx:  "10",
			name: "rogue",
			mask: api.MustParseEventMask("RunPodSandbox"),
		}
	)

	s.Prepare(t,
		&mockRuntime{
			options: []nri.Option{
				// Note: no nri.WithAuthConfig() for this runtime.
				nri.WithTTRPCOptions(nil, []ttrpc.ServerOpt{
					ttrpc.WithUnaryServerInterceptor(recorder.Intercept),
				}),
			},
		},
	)

	s.Startup()

	require.NoError(t, rogue.Connect(s.Dir()))
	t.Cleanup(rogue.Stop)

	_, err := rogue.RequestChallenge(ecdh.Name, []byte("a-made-up-public-key"))

	value, stack := recorder.Recovered()
	require.Nil(t, value, "runtime panicked handling a challenge request: %v\n%s", value, stack)
	require.Error(t, err, "runtime challenged a plugin without authentication configuration")
}

// panicRecorder is a ttrpc server interceptor which turns a panic in a
// request handler into an error, recording the first panic it catches.
type panicRecorder struct {
	sync.Mutex
	value any
	stack []byte
}

// Intercept invokes the given method, recovering and recording a panic.
func (r *panicRecorder) Intercept(ctx context.Context, u ttrpc.Unmarshaler, i *ttrpc.UnaryServerInfo, m ttrpc.Method) (rpl any, err error) {
	defer func() {
		if value := recover(); value != nil {
			r.Lock()
			defer r.Unlock()
			if r.value == nil {
				r.value, r.stack = value, debug.Stack()
			}
			rpl, err = nil, fmt.Errorf("panic in %s: %v", i.FullMethod, value)
		}
	}()

	return m(ctx, u)
}

// Recovered returns the recorded panic value and stack, if any.
func (r *panicRecorder) Recovered() (any, []byte) {
	r.Lock()
	defer r.Unlock()
	return r.value, r.stack
}

// roguePlugin is a plugin which drives connection setup, authentication
// and registration step by step, so that a test can deviate from the
// order and from the steps a well-behaved plugin would perform.
type roguePlugin struct {
	idx       string
	name      string
	publicKey []byte
	mask      api.EventMask

	conn  stdnet.Conn
	mux   multiplex.Mux
	rpcl  stdnet.Listener
	rpcs  *ttrpc.Server
	rpcc  *ttrpc.Client
	syncC chan struct{}
}

var _ = api.PluginService(&roguePlugin{})

// Connect connects the plugin to the runtime and sets up multiplexed
// ttrpc client and server connections, just like the stub does.
func (p *roguePlugin) Connect(dir string) error {
	ctx := context.Background()
	p.syncC = make(chan struct{})

	conn, err := stdnet.Dial("unix", filepath.Join(dir, "nri.sock"))
	if err != nil {
		return err
	}
	p.conn = conn
	p.mux = multiplex.Multiplex(conn)

	if p.rpcl, err = p.mux.Listen(multiplex.PluginServiceConn); err != nil {
		return err
	}
	if p.rpcs, err = ttrpc.NewServer(); err != nil {
		return err
	}

	api.RegisterPluginService(p.rpcs, p)

	go func() {
		_ = p.rpcs.Serve(ctx, p.rpcl)
	}()

	rconn, err := p.mux.Open(multiplex.RuntimeServiceConn)
	if err != nil {
		return err
	}
	p.rpcc = ttrpc.NewClient(rconn)

	return nil
}

// RequestChallenge asks the runtime for an authentication challenge for
// the given algorithm and public key.
func (p *roguePlugin) RequestChallenge(algorithm string, key []byte) (*auth.RequestChallengeResponse, error) {
	return auth.NewAuthenticationClient(p.rpcc).RequestChallenge(context.Background(),
		&auth.RequestChallengeRequest{
			Algorithm: algorithm,
			PublicKey: key,
		},
	)
}

// Register registers the plugin with the runtime.
func (p *roguePlugin) Register() error {
	return p.RegisterAs(p.idx, p.name)
}

// RegisterAs registers the plugin with the runtime using the given
// index and name, which need not be the ones it was set up with.
func (p *roguePlugin) RegisterAs(idx, name string) error {
	return p.RegisterAsWithContext(context.Background(), idx, name)
}

// RegisterAsWithContext is RegisterAs with a caller provided context,
// for a test which needs to bound how long it waits for the runtime.
func (p *roguePlugin) RegisterAsWithContext(ctx context.Context, idx, name string) error {
	_, err := api.NewRuntimeClient(p.rpcc).RegisterPlugin(ctx,
		&api.RegisterPluginRequest{
			PluginName: name,
			PluginIdx:  idx,
		},
	)

	return err
}

// Stop disconnects the plugin from the runtime.
func (p *roguePlugin) Stop() {
	if p.rpcl != nil {
		p.rpcl.Close()
	}
	if p.rpcs != nil {
		p.rpcs.Close()
	}
	if p.rpcc != nil {
		p.rpcc.Close()
	}
	if p.mux != nil {
		p.mux.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// fullName returns the qualified name the runtime knows the plugin by.
func (p *roguePlugin) fullName() string {
	return p.idx + "-" + p.name
}

// Authenticate runs the full authentication handshake with the runtime,
// using the given algorithm and key pair, and returns the result.
func (p *roguePlugin) Authenticate(algorithm string, private, public []byte) (*auth.RespondChallengeResponse, error) {
	impl, err := auth.Get(algorithm)
	if err != nil {
		return nil, err
	}

	algo, err := impl.NewWithKeys(private, public)
	if err != nil {
		return nil, err
	}

	challenge, err := p.RequestChallenge(algorithm, public)
	if err != nil {
		return nil, err
	}

	response, err := algo.Response(challenge.Challenge, auth.PublicKey(challenge.PublicKey))
	if err != nil {
		return nil, err
	}

	return auth.NewAuthenticationClient(p.rpcc).RespondChallenge(context.Background(),
		&auth.RespondChallengeRequest{
			Response: response,
		},
	)
}

// startAuthSuite starts a runtime with the given authentication
// configuration and a validator plugin which records the validation
// requests it receives.
func startAuthSuite(t *testing.T, cfg *auth.Config) (*Suite, chan *api.ValidateContainerAdjustmentRequest) {
	t.Helper()

	var (
		s         = &Suite{}
		validateC = make(chan *api.ValidateContainerAdjustmentRequest, 1)
		validator = &mockPlugin{
			idx:  "00",
			name: "validator",
			mask: api.MustParseEventMask("ValidateContainerAdjustment"),
		}
	)

	s.Prepare(t,
		&mockRuntime{
			options: []nri.Option{
				nri.WithAuthConfig(cfg),
			},
		},
		validator,
	)

	validator.validateAdjustment = func(_ *mockPlugin, req *api.ValidateContainerAdjustmentRequest) error {
		select {
		case validateC <- req:
		default:
		}
		return nil
	}

	s.Startup()

	return s, validateC
}

// validatedPlugins runs a pod and a container with the given id through
// the runtime and returns the plugins the validator was given for the
// container.
func validatedPlugins(t *testing.T, s *Suite, validateC chan *api.ValidateContainerAdjustmentRequest,
	id string) map[string]*api.PluginInstance {
	t.Helper()

	var (
		ctx = context.Background()
		pod = &api.PodSandbox{
			Id:        "pod" + id,
			Name:      "pod" + id,
			Uid:       "uid" + id,
			Namespace: "default",
		}
		ctr = &api.Container{
			Id:           "ctr" + id,
			PodSandboxId: "pod" + id,
			Name:         "ctr" + id,
			State:        api.ContainerState_CONTAINER_CREATED,
		}
	)

	require.NoError(t, s.runtime.RunPodSandbox(ctx, &api.RunPodSandboxRequest{Pod: pod}))
	_, err := s.runtime.CreateContainer(ctx, &api.CreateContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	require.NoError(t, err)

	select {
	case req := <-validateC:
		return req.GetPluginMap()
	case <-time.After(startupTimeout):
		t.Fatal("validator was not invoked")
	}

	return nil
}

// WaitForSync waits for the runtime to synchronize the plugin.
func (p *roguePlugin) WaitForSync(timeout time.Duration) error {
	select {
	case <-p.syncC:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout waiting for plugin synchronization")
	}
}

func (p *roguePlugin) Configure(context.Context, *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	return &api.ConfigureResponse{Events: int32(p.mask)}, nil
}

func (p *roguePlugin) Synchronize(context.Context, *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	select {
	case <-p.syncC:
	default:
		close(p.syncC)
	}
	return &api.SynchronizeResponse{}, nil
}

func (p *roguePlugin) Shutdown(context.Context, *api.Empty) (*api.Empty, error) {
	return &api.Empty{}, nil
}

func (p *roguePlugin) RunPodSandbox(context.Context, *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	return &api.RunPodSandboxResponse{}, nil
}

func (p *roguePlugin) UpdatePodSandbox(context.Context, *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	return &api.UpdatePodSandboxResponse{}, nil
}

func (p *roguePlugin) PostUpdatePodSandbox(context.Context, *api.PostUpdatePodSandboxRequest) (*api.PostUpdatePodSandboxResponse, error) {
	return &api.PostUpdatePodSandboxResponse{}, nil
}

func (p *roguePlugin) StopPodSandbox(context.Context, *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	return &api.StopPodSandboxResponse{}, nil
}

func (p *roguePlugin) RemovePodSandbox(context.Context, *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	return &api.RemovePodSandboxResponse{}, nil
}

func (p *roguePlugin) CreateContainer(context.Context, *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	return &api.CreateContainerResponse{}, nil
}

func (p *roguePlugin) PostCreateContainer(context.Context, *api.PostCreateContainerRequest) (*api.PostCreateContainerResponse, error) {
	return &api.PostCreateContainerResponse{}, nil
}

func (p *roguePlugin) StartContainer(context.Context, *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	return &api.StartContainerResponse{}, nil
}

func (p *roguePlugin) PostStartContainer(context.Context, *api.PostStartContainerRequest) (*api.PostStartContainerResponse, error) {
	return &api.PostStartContainerResponse{}, nil
}

func (p *roguePlugin) UpdateContainer(context.Context, *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	return &api.UpdateContainerResponse{}, nil
}

func (p *roguePlugin) PostUpdateContainer(context.Context, *api.PostUpdateContainerRequest) (*api.PostUpdateContainerResponse, error) {
	return &api.PostUpdateContainerResponse{}, nil
}

func (p *roguePlugin) StopContainer(context.Context, *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	return &api.StopContainerResponse{}, nil
}

func (p *roguePlugin) RemoveContainer(context.Context, *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	return &api.RemoveContainerResponse{}, nil
}

func (p *roguePlugin) StateChange(context.Context, *api.StateChangeEvent) (*api.Empty, error) {
	return &api.Empty{}, nil
}

func (p *roguePlugin) ValidateContainerAdjustment(context.Context, *api.ValidateContainerAdjustmentRequest) (*api.ValidateContainerAdjustmentResponse, error) {
	return &api.ValidateContainerAdjustmentResponse{}, nil
}
