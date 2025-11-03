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
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	nri "github.com/containerd/nri/pkg/adaptation"
	"github.com/containerd/nri/pkg/api/v1alpha1"
	api "github.com/containerd/nri/pkg/api/v1beta1"
	pluginhelpers "github.com/containerd/nri/pkg/plugin"
	validator "github.com/containerd/nri/plugins/default-validator/builtin"

	rspec "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type testbase struct {
	runtime []RuntimeOption
	plugins map[string][]PluginOption
}

func (b *testbase) Plugins(t *testing.T, dir string, events chan<- Event) []*Plugin {
	var (
		ids     = slices.Sorted(maps.Keys(b.plugins))
		plugins = []*Plugin{}
	)

	for _, id := range ids {
		var (
			indexName = strings.SplitN(id, "-", 2)
			options   = append(
				[]PluginOption{
					WithPluginIndex(indexName[0]),
					WithPluginName(indexName[1]),
				},
				b.plugins[id]...,
			)
		)

		plugins = append(plugins, NewPlugin(t, dir, events, options...))
	}

	return plugins
}

func (b *testbase) Setup(t *testing.T) (*Suite, *EventCollector) {
	var (
		dir = t.TempDir()
		evt = StartEventCollector()
		sut = NewSuite(t, dir, evt,
			WithRuntimeOptions(b.runtime...),
			WithPlugins(b.Plugins(t, dir, evt.Channel())...),
		)
	)

	return sut, evt
}

func TestAdaptationSetup(t *testing.T) {
	var (
		syncCB = func(context.Context, nri.SyncCB) error {
			return nil
		}
		updateCB = func(context.Context, []*api.ContainerUpdate) ([]*api.ContainerUpdate, error) {
			return nil, nil
		}
	)

	t.Run("nil runtime synchronize callback", func(t *testing.T) {
		r, err := nri.New(
			TestRuntimeName,
			TestRuntimeVersion,
			nil,
			updateCB,
		)
		require.Nil(t, r, "adaptation creation should return nil adaptation")
		require.NotNil(t, err, "adaptation creation should fail with error")
	})

	t.Run("nil runtime unsolicited plugin update callback", func(t *testing.T) {
		r, err := nri.New(
			TestRuntimeName,
			TestRuntimeVersion,
			syncCB,
			nil,
		)
		require.Nil(t, r, "adaptation creation should return nil adaptation")
		require.NotNil(t, err, "adaptation creation should fail with error")
	})

	t.Run("non-nil runtime callbacks", func(t *testing.T) {
		r, err := nri.New(
			TestRuntimeName,
			TestRuntimeVersion,
			syncCB,
			updateCB,
		)
		require.NotNil(t, r, "adaptation creation should return non-nil adaptation")
		require.Nil(t, err, "adaptation creation should succeed, with nil error")
	})
}

func TestAdaptationConfig(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		verify func(*testing.T)
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	const (
		maxWait = 2 * time.Second
	)

	for _, tc := range []*testcase{
		{
			name: "disable external plugin connections",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDisabledExternalConnections(),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			verify: func(t *testing.T) {
				_, ok := evt.Search(
					OrderedEventsOccurred(
						RuntimeStarted,
						PluginFailed("00-test", os.ErrNotExist),
					),
					UntilTimeout(maxWait),
				)
				require.True(t, ok, "plugin connections should fail")
			},
		},
		{
			name: "enable external plugin connections",
			testbase: &testbase{
				runtime: []RuntimeOption{},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			verify: func(t *testing.T) {
				_, ok := evt.Search(
					OrderedEventsOccurred(
						RuntimeStarted,
						PluginSynchronized("00-test", nil, nil),
					),
					UntilTimeout(maxWait),
				)
				require.True(t, ok, "plugins should be able to connect")
			},
		},
		{
			name: "plugin registration timeout",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithPluginRegistrationTimeout(1 * time.Nanosecond),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			verify: func(t *testing.T) {
				_, ok := evt.Search(
					OrderedEventsOccurred(
						RuntimeStarted,
						PluginClosed("00-test"),
					),
					UntilTimeout(maxWait),
				)
				require.True(t, ok, "plugin connection should be closed")
			},
		},
		{
			name: "plugin request timeout",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithPluginRequestTimeout(100 * time.Microsecond),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						WithHandlers(
							Handlers{
								Configure: func(_, _, _ string) (api.EventMask, error) {
									time.Sleep(110 * time.Millisecond)
									return 0, nil
								},
							},
						),
					},
				},
			},
			verify: func(t *testing.T) {
				_, ok := evt.Search(
					OrderedEventsOccurred(
						RuntimeStarted,
						PluginClosed("00-test"),
					),
					UntilTimeout(maxWait),
				)
				require.True(t, ok, "plugin connection should be closed")
				time.Sleep(2 * time.Second)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start()
			defer func() { sut.Stop(); evt.Stop() }()
			tc.verify(t)
		})
	}
}

func TestPluginConfiguration(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect *PluginEvent
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	const (
		runtimeName     = "custom-runtime-name"
		runtimeVersion  = "v5.6.7"
		registerTimeout = 4 * nri.DefaultPluginRegistrationTimeout / 5
		requestTimeout  = 3 * nri.DefaultPluginRequestTimeout / 4
	)

	for _, tc := range []*testcase{
		{
			name: "with default runtime configuration",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			expect: PluginConfigured(
				"00-test",
				TestRuntimeName,
				TestRuntimeVersion,
				nri.DefaultPluginRegistrationTimeout,
				nri.DefaultPluginRequestTimeout,
			),
		},
		{
			name: "with given runtime name and version",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithRuntimeName(runtimeName),
					WithRuntimeVersion(runtimeVersion),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			expect: PluginConfigured(
				"00-test",
				runtimeName,
				runtimeVersion,
				nri.DefaultPluginRegistrationTimeout,
				nri.DefaultPluginRequestTimeout,
			),
		},
		{
			name: "with given registration and request timeouts",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithPluginRegistrationTimeout(registerTimeout),
					WithPluginRequestTimeout(requestTimeout),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			expect: PluginConfigured(
				"00-test",
				TestRuntimeName,
				TestRuntimeVersion,
				registerTimeout,
				requestTimeout,
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())
			evt.Emit(EndMarker)
			_, occurred := evt.Search(
				EventOccurred(
					tc.expect,
				),
				UntilEndMarker,
			)
			require.True(t, occurred, "plugin configured as expected")
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}
}

func TestEventSubscription(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect []Event
		reject []Event
	}

	var (
		sut *Suite
		evt *EventCollector
		tc  *testcase
	)

	v1beta1Subscribe := func(events ...string) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.ParseEventMask(events...)
				},
			},
		)
	}

	v1alpha1Subscribe := func(events ...string) PluginOption {
		return WithV1alpha1(
			V1alpha1Handlers{
				Configure: func(_, _, _ string) (v1alpha1.EventMask, error) {
					return v1alpha1.ParseEventMask(events...)
				},
			},
		)
	}

	builtinSubscribe := func(events ...string) PluginOption {
		return WithBuiltin(
			BuiltinHandlers{
				Configure: func(_ *api.ConfigureRequest) (*api.ConfigureResponse, error) {
					mask, err := api.ParseEventMask(events...)
					if err != nil {
						return nil, err
					}
					return &api.ConfigureResponse{
						Events: int32(mask),
					}, nil
				},
			},
		)
	}

	for pluginType, subscribe := range map[string]func(events ...string) PluginOption{
		"v1beta1":  v1beta1Subscribe,
		"v1alpha1": v1alpha1Subscribe,
		"builtin":  builtinSubscribe,
	} {
		t.Run(pluginType, func(t *testing.T) {
			for _, tc = range []*testcase{
				{
					name: "RunPodSandbox",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("RunPodSandbox"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginRunPodSandbox("00-test", nil),
					},
					reject: []Event{
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
				},
				{
					name: "UpdatePodSandbox",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("UpdatePodSandbox"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
				},
				{
					name: "PostUpdatePodSandbox",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("PostUpdatePodSandbox"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
				},
				{
					name: "StopPodSandbox",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("StopPodSandbox"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginStopPodSandbox("00-test", nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
				},
				{
					name: "RemovePodSandbox",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("RemovePodSandbox"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginRemovePodSandbox("00-test", nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
					},
				},
				{
					name: "all pod events",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe(
									"RunPodSandbox",
									"UpdatePodSandbox",
									"PostUpdatePodSandbox",
									"StopPodSandbox",
									"RemovePodSandbox",
								),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
					reject: []Event{
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "CreateContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("CreateContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginCreateContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "PostCreateContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("PostCreateContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "StartContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("StartContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "PostStartContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("PostStartContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "UpdateContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("UpdateContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "PostUpdateContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("PostUpdateContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "StopContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("StopContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
				{
					name: "RemoveContainer",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe("RemoveContainer"),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
					},
				},
				{
					name: "all container events",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe(
									"CreateContainer",
									"PostCreateContainer",
									"StartContainer",
									"PostStartContainer",
									"UpdateContainer",
									"PostUpdateContainer",
									"StopContainer",
									"RemoveContainer",
								),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
					reject: []Event{
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
					},
				},
				{
					name: "all pod and container events",
					testbase: &testbase{
						plugins: map[string][]PluginOption{
							"00-test": {
								subscribe(
									"RunPodSandbox",
									"UpdatePodSandbox",
									"PostUpdatePodSandbox",
									"StopPodSandbox",
									"RemovePodSandbox",
									"CreateContainer",
									"PostCreateContainer",
									"StartContainer",
									"PostStartContainer",
									"UpdateContainer",
									"PostUpdateContainer",
									"StopContainer",
									"RemoveContainer",
								),
							},
						},
					},
					expect: []Event{
						PluginConfigured("00-test", "", "", 0, 0),
						PluginSynchronized("00-test", nil, nil),
						PluginRunPodSandbox("00-test", nil),
						PluginUpdatePodSandbox("00-test", nil, nil, nil),
						PluginPostUpdatePodSandbox("00-test", nil),
						PluginStopPodSandbox("00-test", nil),
						PluginRemovePodSandbox("00-test", nil),
						PluginCreateContainer("00-test", nil, nil),
						PluginPostCreateContainer("00-test", nil, nil),
						PluginStartContainer("00-test", nil, nil),
						PluginPostStartContainer("00-test", nil, nil),
						PluginUpdateContainer("00-test", nil, nil, nil),
						PluginPostUpdateContainer("00-test", nil, nil),
						PluginStopContainer("00-test", nil, nil),
						PluginRemoveContainer("00-test", nil, nil),
					},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					sut, evt = tc.Setup(t)
					sut.Start(WithWaitForPluginsToStart())

					pod := sut.NewPod(
						WithPodRandomFill(),
						WithPodName("pod0"),
					)

					ctr := sut.NewContainer(
						WithContainerRandomFill(),
						WithContainerPod(pod),
						WithContainerName("ctr0"),
					)

					require.NoError(t, sut.StartUpdateStopPodAndContainer(pod, ctr))
					evt.Emit(EndMarker)

					for _, expected := range tc.expect {
						_, occurred := evt.Search(
							EventOccurred(expected),
							UntilEndMarker,
						)
						require.True(t, occurred, "expected event %s occurred", expected)
					}

					for _, rejected := range tc.reject {
						_, occurred := evt.Search(
							EventOccurred(rejected),
							UntilEndMarker,
						)
						require.False(t, occurred, "unexpected event %s occurred", rejected)
					}

					require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
					evt.Stop()
				})
			}
		})
	}
}

func TestContainerAdjustment(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect *api.ContainerAdjustment
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	adjust := func(fn func(*api.ContainerAdjustment)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					fn(a)
					return a, nil, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "adjust annotations",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("key", "00-test")
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Annotations: map[string]string{
					"key": "00-test",
				},
			},
		},
		{
			name: "adjust mounts",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddMount(&api.Mount{
								Source:      "/dev/00-test",
								Destination: "/mnt/test",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Mounts: []*api.Mount{
					{
						Source:      "/dev/00-test",
						Destination: "/mnt/test",
					},
				},
			},
		},
		{
			name: "adjust environment",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddEnv("00-test", "true")
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Env: []*api.KeyValue{
					{
						Key:   "00-test",
						Value: "true",
					},
				},
			},
		},
		{
			name: "adjust OCI hooks",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddHooks(&api.Hooks{
								CreateRuntime: []*api.Hook{
									{
										Path: "/bin/00-test",
										Args: []string{"/bin/00-test", "arg1", "arg2"},
									},
								},
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Hooks: &api.Hooks{
					CreateRuntime: []*api.Hook{
						{
							Path: "/bin/00-test",
							Args: []string{"/bin/00-test", "arg1", "arg2"},
						},
					},
				},
			},
		},
		{
			name: "adjust Linux devices",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "c",
								Major:    1,
								Minor:    2,
								FileMode: api.FileMode(0o644),
								Uid:      api.UInt32(1000),
								Gid:      api.UInt32(1001),
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Devices: []*api.LinuxDevice{
						{
							Path:     "/dev/00-test",
							Type:     "c",
							Major:    1,
							Minor:    2,
							FileMode: api.FileMode(0o644),
							Uid:      api.UInt32(1000),
							Gid:      api.UInt32(1001),
						},
					},
				},
			},
		},
		{
			name: "adjust Linux resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxMemoryLimit(1)
							a.SetLinuxMemoryReservation(2)
							a.SetLinuxMemorySwap(3)
							a.SetLinuxMemoryKernel(4)
							a.SetLinuxMemoryKernelTCP(5)
							a.SetLinuxMemorySwappiness(6)
							a.SetLinuxMemoryDisableOomKiller()
							a.SetLinuxMemoryUseHierarchy()
							a.SetLinuxCPUShares(10)
							a.SetLinuxCPUQuota(20)
							a.SetLinuxCPUPeriod(30)
							a.SetLinuxCPURealtimeRuntime(40)
							a.SetLinuxCPURealtimePeriod(50)
							a.SetLinuxCPUSetCPUs("0-3")
							a.SetLinuxCPUSetMems("1")
							a.AddLinuxHugepageLimit("2MB", 1024)
							a.AddLinuxHugepageLimit("1GB", 2048)
							a.SetLinuxBlockIOClass("blockio-class1")
							a.SetLinuxRDTClass("rdt-class1")
							a.AddLinuxUnified("key1", "value1")
							a.AddLinuxUnified("key2", "value2")
							// FIXME: no convenience method for cgroup device rules
							a.Linux.Resources.Devices = []*api.LinuxDeviceCgroup{
								{
									Allow:  true,
									Type:   "c",
									Major:  api.Int64(1),
									Minor:  api.Int64(2),
									Access: "rwm",
								},
								{
									Allow:  false,
									Type:   "b",
									Major:  api.Int64(3),
									Minor:  api.Int64(4),
									Access: "rm",
								},
							}
							a.SetLinuxPidLimits(100)
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Resources: &api.LinuxResources{
						Memory: &api.LinuxMemory{
							Limit:            api.Int64(1),
							Reservation:      api.Int64(2),
							Swap:             api.Int64(3),
							Kernel:           api.Int64(4),
							KernelTcp:        api.Int64(5),
							Swappiness:       api.UInt64(6),
							DisableOomKiller: api.Bool(true),
							UseHierarchy:     api.Bool(true),
						},
						Cpu: &api.LinuxCPU{
							Shares:          api.UInt64(10),
							Quota:           api.Int64(20),
							Period:          api.UInt64(30),
							RealtimeRuntime: api.Int64(40),
							RealtimePeriod:  api.UInt64(50),
							Cpus:            "0-3",
							Mems:            "1",
						},
						HugepageLimits: []*api.HugepageLimit{
							{
								PageSize: "2MB",
								Limit:    1024,
							},
							{
								PageSize: "1GB",
								Limit:    2048,
							},
						},
						BlockioClass: api.String("blockio-class1"),
						RdtClass:     api.String("rdt-class1"),
						Unified: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
						Devices: []*api.LinuxDeviceCgroup{
							{
								Allow:  true,
								Type:   "c",
								Major:  api.Int64(1),
								Minor:  api.Int64(2),
								Access: "rwm",
							},
							{
								Allow:  false,
								Type:   "b",
								Major:  api.Int64(3),
								Minor:  api.Int64(4),
								Access: "rm",
							},
						},
						Pids: &api.LinuxPids{
							Limit: 100,
						},
					},
				},
			},
		},
		{
			name: "adjust cgroups path",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxCgroupsPath("/sys/fs/cgroups/00-test")
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					CgroupsPath: "/sys/fs/cgroups/00-test",
				},
			},
		},
		{
			name: "adjust OOM score adjustment",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							v := 987
							a.SetLinuxOomScoreAdj(&v)
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					OomScoreAdj: api.Int(987),
				},
			},
		},
		{
			name: "adjust I/O priority",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxIOPriority(&nri.LinuxIOPriority{
								Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
								Priority: 5,
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					IoPriority: &api.LinuxIOPriority{
						Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
						Priority: 5,
					},
				},
			},
		},
		{
			name: "adjust seccomp policy",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					SeccompPolicy: func() *api.LinuxSeccomp {
						seccomp := rspec.LinuxSeccomp{
							DefaultAction: rspec.ActAllow,
							ListenerPath:  "/run/meshuggah-rocks.sock",
							Architectures: []rspec.Arch{},
							Flags:         []rspec.LinuxSeccompFlag{},
							Syscalls: []rspec.LinuxSyscall{{
								Names:  []string{"sched_getaffinity"},
								Action: rspec.ActNotify,
								Args:   []rspec.LinuxSeccompArg{},
							}},
						}
						return api.FromOCILinuxSeccomp(&seccomp)
					}(),
				},
			},
		},
		{
			name: "adjust namespaces",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(&nri.LinuxNamespace{
								Type: "cgroup",
								Path: "/tmp/00-test",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Namespaces: []*api.LinuxNamespace{
						{
							Type: "cgroup",
							Path: "/tmp/00-test",
						},
					},
				},
			},
		},
		{
			name: "adjust POSIX rlimits",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddRlimit("nofile", 456, 123)
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Rlimits: []*api.POSIXRlimit{{Type: "nofile", Soft: 123, Hard: 456}},
			},
		},
		{
			name: "adjust CDI devices",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddCDIDevice(
								&api.CDIDevice{
									Name: "vendor0.com/dev=dev0",
								},
							)
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				CDIDevices: []*api.CDIDevice{
					{
						Name: "vendor0.com/dev=dev0",
					},
				},
			},
		},
		{
			name: "adjust arguments",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetArgs([]string{
								"echo",
								"updated",
								"argument",
								"list",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Args: []string{
					"echo",
					"updated",
					"argument",
					"list",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			adjust, _, err := sut.CreateContainer(pod, ctr)
			require.NoError(t, err, "CreateContainer should succeed")
			require.True(t,
				protoEqual(tc.expect.Strip(), adjust.Strip()),
				"container adjustment matches expected: %s",
				protoDiff(tc.expect.Strip(), adjust.Strip()),
			)
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}
}

func TestContainerAdjustmentConflictDetection(t *testing.T) {
	type testcase struct {
		*testbase
		name string
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	adjust := func(fn func(*api.ContainerAdjustment)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					fn(a)
					return a, nil, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "adjust annotations",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("key", "00-test")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("key", "01-test")
						}),
					},
				},
			},
		},
		{
			name: "adjust mounts",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddMount(&api.Mount{
								Source:      "/dev/00-test",
								Destination: "/mnt/test",
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddMount(&api.Mount{
								Source:      "/dev/01-test",
								Destination: "/mnt/test",
							})
						}),
					},
				},
			},
		},
		{
			name: "adjust environment",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddEnv("00-test", "true")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddEnv("00-test", "false")
						}),
					},
				},
			},
		},
		// no test for OCI hooks: they are collected unconditionally, without conflicts
		{
			name: "adjust Linux devices",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "c",
								Major:    1,
								Minor:    2,
								FileMode: api.FileMode(0o644),
								Uid:      api.UInt32(1000),
								Gid:      api.UInt32(1001),
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "b",
								Major:    3,
								Minor:    4,
								FileMode: api.FileMode(0o600),
								Uid:      api.UInt32(2000),
								Gid:      api.UInt32(2001),
							})
						}),
					},
				},
			},
		},
		{
			name: "adjust Linux resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxMemoryLimit(1)
							a.SetLinuxMemoryReservation(2)
							a.SetLinuxMemorySwap(3)
							a.SetLinuxMemoryKernel(4)
							a.SetLinuxMemoryKernelTCP(5)
							a.SetLinuxMemorySwappiness(6)
							a.SetLinuxMemoryDisableOomKiller()
							a.SetLinuxMemoryUseHierarchy()
							a.SetLinuxCPUShares(10)
							a.SetLinuxCPUQuota(20)
							a.SetLinuxCPUPeriod(30)
							a.SetLinuxCPURealtimeRuntime(40)
							a.SetLinuxCPURealtimePeriod(50)
							a.SetLinuxCPUSetCPUs("0-3")
							a.SetLinuxCPUSetMems("1")
							a.AddLinuxHugepageLimit("2MB", 1024)
							a.AddLinuxHugepageLimit("1GB", 2048)
							a.SetLinuxBlockIOClass("blockio-class1")
							a.SetLinuxRDTClass("rdt-class1")
							a.AddLinuxUnified("key1", "value1")
							a.AddLinuxUnified("key2", "value2")
							// FIXME: no convenience method for cgroup device rules
							a.Linux.Resources.Devices = []*api.LinuxDeviceCgroup{
								{
									Allow:  true,
									Type:   "c",
									Major:  api.Int64(1),
									Minor:  api.Int64(2),
									Access: "rwm",
								},
								{
									Allow:  false,
									Type:   "b",
									Major:  api.Int64(3),
									Minor:  api.Int64(4),
									Access: "rm",
								},
							}
							a.SetLinuxPidLimits(100)
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxCPUSetCPUs("0")
							a.SetLinuxCPUSetMems("0")
						}),
					},
				},
			},
		},
		{
			name: "adjust cgroups path",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxCgroupsPath("/sys/fs/cgroups/00-test")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxCgroupsPath("/sys/fs/cgroups/01-test")
						}),
					},
				},
			},
		},
		{
			name: "adjust OOM score adjustment",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							v := 987
							a.SetLinuxOomScoreAdj(&v)
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							v := 986
							a.SetLinuxOomScoreAdj(&v)
						}),
					},
				},
			},
		},
		{
			name: "adjust I/O priority",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxIOPriority(&nri.LinuxIOPriority{
								Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
								Priority: 5,
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxIOPriority(&nri.LinuxIOPriority{
								Class:    api.IOPrioClass_IOPRIO_CLASS_IDLE,
								Priority: 1,
							})
						}),
					},
				},
			},
		},
		{
			name: "adjust seccomp policy",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/megadeth-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_setaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
		},
		{
			name: "adjust namespaces",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(&nri.LinuxNamespace{
								Type: "cgroup",
								Path: "/tmp/00-test",
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(&nri.LinuxNamespace{
								Type: "cgroup",
								Path: "/tmp/01-test",
							})
						}),
					},
				},
			},
		},
		{
			name: "adjust POSIX rlimits",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddRlimit("nofile", 456, 123)
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddRlimit("nofile", 789, 456)
						}),
					},
				},
			},
		},
		{
			name: "adjust CDI devices",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddCDIDevice(
								&api.CDIDevice{
									Name: "vendor0.com/dev=dev0",
								},
							)
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddCDIDevice(
								&api.CDIDevice{
									Name: "vendor0.com/dev=dev0",
								},
							)
						}),
					},
				},
			},
		},
		{
			name: "adjust arguments",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetArgs([]string{
								"echo",
								"updated",
								"argument",
								"list",
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetArgs([]string{
								"echo",
								"another",
								"updated",
								"argument",
								"list",
							})
						}),
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			adjust, _, err := sut.CreateContainer(pod, ctr)
			require.Nil(t, adjust, "CreateContainer should fail")
			require.Error(t, err, "CreateContainer should fail")
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}

}

func TestContainerAdjustmentConflictAvoidance(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect *api.ContainerAdjustment
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	adjust := func(fn func(*api.ContainerAdjustment)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					fn(a)
					return a, nil, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "adjust annotations",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("key", "00-test")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.RemoveAnnotation("key")
							a.AddAnnotation("key", "01-test")
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Annotations: map[string]string{
					// TODO(klihub): do we need/want to leave the removal marker ?
					// Do we want to leave it if the same key is not re-set, so that
					// it can be used to remove an annotation from the final container ?
					api.MarkForRemoval("key"): "",
					"key":                     "01-test",
				},
			},
		},
		{
			name: "adjust mounts",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddMount(&api.Mount{
								Source:      "/dev/00-test",
								Destination: "/mnt/test",
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.RemoveMount("/mnt/test1")
							a.RemoveMount("/mnt/test")
							a.AddMount(&api.Mount{
								Source:      "/dev/01-test",
								Destination: "/mnt/test",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Mounts: []*api.Mount{
					{
						Source:      "/dev/01-test",
						Destination: "/mnt/test",
					},
					{
						Destination: api.MarkForRemoval("/mnt/test1"),
					},
				},
			},
		},
		{
			name: "adjust environment",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddEnv("00-test", "true")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							// TODO(klihub): This seems now inconsistent with
							// annotations and mounts. Should we leave a removal
							// marker in the collected adjustments if the same
							// variable has not been re-set ?
							// a.RemoveEnv("00-test1")
							a.RemoveEnv("00-test")
							a.AddEnv("00-test", "01-test")
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Env: []*api.KeyValue{
					{
						Key:   "00-test",
						Value: "01-test",
					},
				},
			},
		},
		// no test for OCI hooks: they are collected unconditionally, without conflicts
		{
			name: "adjust Linux devices",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "c",
								Major:    1,
								Minor:    2,
								FileMode: api.FileMode(0o644),
								Uid:      api.UInt32(1000),
								Gid:      api.UInt32(1001),
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							// TODO(klihub): this seems now inconsistent with mounts.
							// Should we leave a removal marker in the collected adjustments
							// if the same device has not been re-added ?
							a.RemoveDevice("/dev/01-test")
							a.RemoveDevice("/dev/00-test")
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "b",
								Major:    3,
								Minor:    4,
								FileMode: api.FileMode(0o600),
								Uid:      api.UInt32(2000),
								Gid:      api.UInt32(2001),
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Linux: &api.LinuxContainerAdjustment{
					Devices: []*api.LinuxDevice{
						{
							Path:     "/dev/00-test",
							Type:     "b",
							Major:    3,
							Minor:    4,
							FileMode: api.FileMode(0o600),
							Uid:      api.UInt32(2000),
							Gid:      api.UInt32(2001),
						},
					},
				},
			},
		},
		// no test for Linux resources: multiple adjustments always conflict
		// no test for cgroups path: multiple adjustments always conflict
		// no test for OOM score adjustment: multiple adjustments always conflict
		// no test for I/O priority: multiple adjustments always conflict
		// no test for seccomp policy: multiple adjustments always conflict
		// no test for namespaces: multiple adjustments always conflict
		// no test for POSIX rlimits: multiple adjustments always conflict
		// no test for CDI devices: multiple adjustments always conflict
		{
			name: "adjust arguments",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetArgs([]string{
								"echo",
								"updated",
								"argument",
								"list",
							})
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.UpdateArgs([]string{
								"echo",
								"another",
								"updated",
								"argument",
								"list",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Args: []string{
					"echo",
					"another",
					"updated",
					"argument",
					"list",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			adjust, _, err := sut.CreateContainer(pod, ctr)
			require.NoError(t, err, "CreateContainer should succeed")
			require.True(t,
				protoEqual(tc.expect.Strip(), adjust.Strip()),
				"container adjustment matches expected: %s",
				protoDiff(tc.expect.Strip(), adjust.Strip()),
			)

			require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
			evt.Stop()
		})
	}
}

func TestContainerAdjustmentCollection(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect *api.ContainerAdjustment
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	adjust := func(fn func(*api.ContainerAdjustment)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					fn(a)
					return a, nil, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "adjust everything in a container, by one plugin each",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("key", "00-test")
						}),
					},
					"01-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddMount(&api.Mount{
								Source:      "/dev/00-test",
								Destination: "/mnt/test",
							})
						}),
					},
					"02-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddEnv("00-test", "true")
						}),
					},
					"03-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddHooks(&api.Hooks{
								CreateRuntime: []*api.Hook{
									{
										Path: "/bin/00-test",
										Args: []string{"/bin/00-test", "arg1", "arg2"},
									},
								},
							})
						}),
					},
					"04-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddDevice(&api.LinuxDevice{
								Path:     "/dev/00-test",
								Type:     "c",
								Major:    1,
								Minor:    2,
								FileMode: api.FileMode(0o644),
								Uid:      api.UInt32(1000),
								Gid:      api.UInt32(1001),
							})
						}),
					},
					"05-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxMemoryLimit(1)
							a.SetLinuxMemoryReservation(2)
							a.SetLinuxMemorySwap(3)
							a.SetLinuxMemoryKernel(4)
							a.SetLinuxMemoryKernelTCP(5)
							a.SetLinuxMemorySwappiness(6)
							a.SetLinuxMemoryDisableOomKiller()
							a.SetLinuxMemoryUseHierarchy()
							a.SetLinuxCPUShares(10)
							a.SetLinuxCPUQuota(20)
							a.SetLinuxCPUPeriod(30)
							a.SetLinuxCPURealtimeRuntime(40)
							a.SetLinuxCPURealtimePeriod(50)
							a.SetLinuxCPUSetCPUs("0-3")
							a.SetLinuxCPUSetMems("1")
							a.AddLinuxHugepageLimit("2MB", 1024)
							a.AddLinuxHugepageLimit("1GB", 2048)
							a.SetLinuxBlockIOClass("blockio-class1")
							a.SetLinuxRDTClass("rdt-class1")
							a.AddLinuxUnified("key1", "value1")
							a.AddLinuxUnified("key2", "value2")
							// FIXME: no convenience method for cgroup device rules
							a.Linux.Resources.Devices = []*api.LinuxDeviceCgroup{
								{
									Allow:  true,
									Type:   "c",
									Major:  api.Int64(1),
									Minor:  api.Int64(2),
									Access: "rwm",
								},
								{
									Allow:  false,
									Type:   "b",
									Major:  api.Int64(3),
									Minor:  api.Int64(4),
									Access: "rm",
								},
							}
							a.SetLinuxPidLimits(100)
						}),
					},
					"06-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxCgroupsPath("/sys/fs/cgroups/00-test")
						}),
					},
					"07-test": {
						adjust(func(a *api.ContainerAdjustment) {
							v := 987
							a.SetLinuxOomScoreAdj(&v)
						}),
					},
					"08-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxIOPriority(&nri.LinuxIOPriority{
								Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
								Priority: 5,
							})
						}),
					},
					"09-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
					"10-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(&nri.LinuxNamespace{
								Type: "cgroup",
								Path: "/tmp/00-test",
							})
						}),
					},
					"11-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddRlimit("nofile", 456, 123)
						}),
					},
					"12-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddCDIDevice(
								&api.CDIDevice{
									Name: "vendor0.com/dev=dev0",
								},
							)
						}),
					},
					"13-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetArgs([]string{
								"echo",
								"updated",
								"argument",
								"list",
							})
						}),
					},
				},
			},
			expect: &api.ContainerAdjustment{
				Annotations: map[string]string{
					"key": "00-test",
				},
				Mounts: []*api.Mount{
					{
						Source:      "/dev/00-test",
						Destination: "/mnt/test",
					},
				},
				Env: []*api.KeyValue{
					{
						Key:   "00-test",
						Value: "true",
					},
				},
				Hooks: &api.Hooks{
					CreateRuntime: []*api.Hook{
						{
							Path: "/bin/00-test",
							Args: []string{"/bin/00-test", "arg1", "arg2"},
						},
					},
				},
				Linux: &api.LinuxContainerAdjustment{
					Devices: []*api.LinuxDevice{
						{
							Path:     "/dev/00-test",
							Type:     "c",
							Major:    1,
							Minor:    2,
							FileMode: api.FileMode(0o644),
							Uid:      api.UInt32(1000),
							Gid:      api.UInt32(1001),
						},
					},
					Resources: &api.LinuxResources{
						Memory: &api.LinuxMemory{
							Limit:            api.Int64(1),
							Reservation:      api.Int64(2),
							Swap:             api.Int64(3),
							Kernel:           api.Int64(4),
							KernelTcp:        api.Int64(5),
							Swappiness:       api.UInt64(6),
							DisableOomKiller: api.Bool(true),
							UseHierarchy:     api.Bool(true),
						},
						Cpu: &api.LinuxCPU{
							Shares:          api.UInt64(10),
							Quota:           api.Int64(20),
							Period:          api.UInt64(30),
							RealtimeRuntime: api.Int64(40),
							RealtimePeriod:  api.UInt64(50),
							Cpus:            "0-3",
							Mems:            "1",
						},
						HugepageLimits: []*api.HugepageLimit{
							{
								PageSize: "2MB",
								Limit:    1024,
							},
							{
								PageSize: "1GB",
								Limit:    2048,
							},
						},
						BlockioClass: api.String("blockio-class1"),
						RdtClass:     api.String("rdt-class1"),
						Unified: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
						Devices: []*api.LinuxDeviceCgroup{
							{
								Allow:  true,
								Type:   "c",
								Major:  api.Int64(1),
								Minor:  api.Int64(2),
								Access: "rwm",
							},
							{
								Allow:  false,
								Type:   "b",
								Major:  api.Int64(3),
								Minor:  api.Int64(4),
								Access: "rm",
							},
						},
						Pids: &api.LinuxPids{
							Limit: 100,
						},
					},
					CgroupsPath: "/sys/fs/cgroups/00-test",
					OomScoreAdj: api.Int(987),
					IoPriority: &api.LinuxIOPriority{
						Class:    api.IOPrioClass_IOPRIO_CLASS_RT,
						Priority: 5,
					},
					SeccompPolicy: func() *api.LinuxSeccomp {
						seccomp := rspec.LinuxSeccomp{
							DefaultAction: rspec.ActAllow,
							ListenerPath:  "/run/meshuggah-rocks.sock",
							Architectures: []rspec.Arch{},
							Flags:         []rspec.LinuxSeccompFlag{},
							Syscalls: []rspec.LinuxSyscall{{
								Names:  []string{"sched_getaffinity"},
								Action: rspec.ActNotify,
								Args:   []rspec.LinuxSeccompArg{},
							}},
						}
						return api.FromOCILinuxSeccomp(&seccomp)
					}(),
					Namespaces: []*api.LinuxNamespace{
						{
							Type: "cgroup",
							Path: "/tmp/00-test",
						},
					},
				},
				Rlimits: []*api.POSIXRlimit{{Type: "nofile", Soft: 123, Hard: 456}},
				CDIDevices: []*api.CDIDevice{
					{
						Name: "vendor0.com/dev=dev0",
					},
				},
				Args: []string{
					"echo",
					"updated",
					"argument",
					"list",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			adjust, _, err := sut.CreateContainer(pod, ctr)
			require.NoError(t, err, "CreateContainer should succeed")
			require.True(t,
				protoEqual(tc.expect.Strip(), adjust.Strip()),
				"container adjustment matches expected: %s",
				protoDiff(tc.expect.Strip(), adjust.Strip()),
			)
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}

}

func TestSolicitedContainerUpdates(t *testing.T) {
	type testcase struct {
		*testbase
		name   string
		expect *api.ContainerUpdate
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	update := func(fn func(string, *api.ContainerUpdate)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					u := &api.ContainerUpdate{}
					fn(ctr.Name, u)
					return nil, []*api.ContainerUpdate{u}, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "update Linux CPU resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxCPUShares(123)
								u.SetLinuxCPUQuota(456)
								u.SetLinuxCPUPeriod(789)
								u.SetLinuxCPURealtimeRuntime(321)
								u.SetLinuxCPURealtimePeriod(654)
								u.SetLinuxCPUSetCPUs("0-1")
								u.SetLinuxCPUSetMems("2-3")
							}
						}),
					},
				},
			},
			expect: &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						Cpu: &api.LinuxCPU{
							Shares:          api.UInt64(123),
							Quota:           api.Int64(456),
							Period:          api.UInt64(789),
							RealtimeRuntime: api.Int64(321),
							RealtimePeriod:  api.UInt64(654),
							Cpus:            "0-1",
							Mems:            "2-3",
						},
					},
				},
			},
		},
		{
			name: "update Linux memory resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxMemoryLimit(1234)
								u.SetLinuxMemoryReservation(4567)
								u.SetLinuxMemorySwap(7890)
								u.SetLinuxMemoryKernel(9012)
								u.SetLinuxMemoryKernelTCP(2345)
								u.SetLinuxMemorySwappiness(5678)
								u.SetLinuxMemoryDisableOomKiller()
								u.SetLinuxMemoryUseHierarchy()
							}
						}),
					},
				},
			},
			expect: &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						Memory: &api.LinuxMemory{
							Limit:            api.Int64(1234),
							Reservation:      api.Int64(4567),
							Swap:             api.Int64(7890),
							Kernel:           api.Int64(9012),
							KernelTcp:        api.Int64(2345),
							Swappiness:       api.UInt64(5678),
							DisableOomKiller: api.Bool(true),
							UseHierarchy:     api.Bool(true),
						},
					},
				},
			},
		},
		{
			name: "update Linux class-based resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxRDTClass("00-test.rdt")
								u.SetLinuxBlockIOClass("00-test.blkio")
							}
						}),
					},
				},
			},
			expect: &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						RdtClass:     api.String("00-test.rdt"),
						BlockioClass: api.String("00-test.blkio"),
					},
				},
			},
		},
		{
			name: "update Linux hugepage limits",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxHugepageLimit("1M", 4096)
								u.AddLinuxHugepageLimit("4M", 1024)
							}
						}),
					},
				},
			},
			expect: &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						HugepageLimits: []*api.HugepageLimit{
							{
								PageSize: "1M",
								Limit:    4096,
							},
							{
								PageSize: "4M",
								Limit:    1024,
							},
						},
					},
				},
			},
		},
		{
			name: "update Linux cgroupv2 unified resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxUnified("resource.1", "value1")
								u.AddLinuxUnified("resource.2", "value2")
							}
						}),
					},
				},
			},
			expect: &api.ContainerUpdate{
				Linux: &api.LinuxContainerUpdate{
					Resources: &api.LinuxResources{
						Unified: map[string]string{
							"resource.1": "value1",
							"resource.2": "value2",
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr0 := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			_, _, err := sut.CreateContainer(pod, ctr0)
			require.NoError(t, err, "CreateContainer should succeed")

			ctr1 := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr1"),
			)

			_, updates, err := sut.CreateContainer(pod, ctr1)
			require.NoError(t, err, "CreateContainer should succeed")

			tc.expect.ContainerId = updates[0].ContainerId
			require.True(t,
				protoEqual(tc.expect.Strip(), updates[0].Strip()),
				"container adjustment matches expected: %s",
				protoDiff(tc.expect.Strip(), updates[0].Strip()),
			)
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}
}

func TestSolicitedContainerUpdatesConflictDetection(t *testing.T) {
	type testcase struct {
		*testbase
		name string
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	update := func(fn func(string, *api.ContainerUpdate)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					u := &api.ContainerUpdate{}
					fn(ctr.Name, u)
					return nil, []*api.ContainerUpdate{u}, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "update Linux CPU resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxCPUShares(123)
								u.SetLinuxCPUQuota(456)
								u.SetLinuxCPUPeriod(789)
								u.SetLinuxCPURealtimeRuntime(321)
								u.SetLinuxCPURealtimePeriod(654)
								u.SetLinuxCPUSetCPUs("0-1")
								u.SetLinuxCPUSetMems("2-3")
							}
						}),
					},
					"01-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxCPUShares(123)
							}
						}),
					},
				},
			},
		},
		{
			name: "update Linux memory resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxMemoryLimit(1234)
								u.SetLinuxMemoryReservation(4567)
								u.SetLinuxMemorySwap(7890)
								u.SetLinuxMemoryKernel(9012)
								u.SetLinuxMemoryKernelTCP(2345)
								u.SetLinuxMemorySwappiness(5678)
								u.SetLinuxMemoryDisableOomKiller()
								u.SetLinuxMemoryUseHierarchy()
							}
						}),
					},
					"01-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxMemoryLimit(1234)
							}
						}),
					},
				},
			},
		},
		{
			name: "update Linux class-based resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxRDTClass("00-test.rdt")
								u.SetLinuxBlockIOClass("00-test.blkio")
							}
						}),
					},
					"01-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.SetLinuxRDTClass("00-test.rdt")
								u.SetLinuxBlockIOClass("00-test.blkio")
							}
						}),
					},
				},
			},
		},
		{
			name: "update Linux hugepage limits",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxHugepageLimit("1M", 4096)
								u.AddLinuxHugepageLimit("4M", 1024)
							}
						}),
					},
					"01-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxHugepageLimit("1M", 4096)
							}
						}),
					},
				},
			},
		},
		{
			name: "update Linux cgroupv2 unified resources",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxUnified("resource.1", "value1")
								u.AddLinuxUnified("resource.2", "value2")
							}
						}),
					},
					"01-test": {
						update(func(ctr string, u *api.ContainerUpdate) {
							if ctr == "ctr1" {
								u.AddLinuxUnified("resource.1", "value1")
							}
						}),
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				WithPodRandomFill(),
				WithPodName("pod0"),
			)

			ctr0 := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr0"),
			)

			_, _, err := sut.CreateContainer(pod, ctr0)
			require.NoError(t, err, "CreateContainer should succeed")

			ctr1 := sut.NewContainer(
				WithContainerRandomFill(),
				WithContainerPod(pod),
				WithContainerName("ctr1"),
			)

			adjust, updates, err := sut.CreateContainer(pod, ctr1)
			require.Nil(t, adjust, "CreateContainer should fail")
			require.Nil(t, updates, "CreateContainer should fail")
			require.Error(t, err, "CreateContainer should fail")
		})

		require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
		evt.Stop()
	}
}

func TestContainerAdjustmentValidation(t *testing.T) {
	type testcase struct {
		*testbase
		name      string
		pod       []PodOption
		container []ContainerOption
		fail      bool
	}

	var (
		sut *Suite
		evt *EventCollector
	)

	validate := func(fn func(req *api.ValidateContainerAdjustmentRequest) error) PluginOption {
		return WithHandlers(
			Handlers{
				ValidateContainerAdjustment: fn,
			},
		)
	}

	adjust := func(fn func(*api.ContainerAdjustment)) PluginOption {
		return WithHandlers(
			Handlers{
				Configure: func(_, _, _ string) (api.EventMask, error) {
					return api.Event_CREATE_CONTAINER.Mask(), nil
				},
				CreateContainer: func(_ *api.PodSandbox, _ *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
					a := &api.ContainerAdjustment{}
					fn(a)
					return a, nil, nil
				},
			},
		)
	}

	for _, tc := range []*testcase{
		{
			name: "pass custom external validation",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("allowed", "00-test")
						}),
					},
					"01-validator": {
						validate(func(_ *api.ValidateContainerAdjustmentRequest) error {
							return nil
						}),
					},
				},
			},
		},
		{
			name: "fail custom external validation",
			testbase: &testbase{
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddAnnotation("forbidden", "00-test")
						}),
					},
					"01-validator": {
						validate(func(req *api.ValidateContainerAdjustmentRequest) error {
							_, reject := req.Owners.AnnotationOwner(
								req.Container.Id,
								"forbidden",
							)
							if reject {
								return fmt.Errorf("forbidden annotation set")
							}
							return nil
						}),
					},
				},
			},
			fail: true,
		},
		{
			name: "reject OCI hook injection",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                  true,
								RejectOCIHookAdjustment: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddHooks(
								&api.Hooks{
									Prestart: []*api.Hook{
										{
											Path: "/bin/sh",
											Args: []string{"/bin/sh", "-c", "true"},
										},
									},
								},
							)
						}),
					},
				},
			},
			fail: true,
		},
		{
			name: "reject default seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                                true,
								RejectRuntimeDefaultSeccompAdjustment: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType: api.SecurityProfile_RUNTIME_DEFAULT,
					},
				),
			},
			fail: true,
		},
		{
			name: "allow default seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                                true,
								RejectRuntimeDefaultSeccompAdjustment: false,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType: api.SecurityProfile_RUNTIME_DEFAULT,
					},
				),
			},
		},
		{
			name: "reject custom seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                        true,
								RejectCustomSeccompAdjustment: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType:  api.SecurityProfile_LOCALHOST,
						LocalhostRef: "/xyzzy/foobar",
					},
				),
			},
			fail: true,
		},
		{
			name: "allow custom seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                        true,
								RejectCustomSeccompAdjustment: false,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType:  api.SecurityProfile_LOCALHOST,
						LocalhostRef: "/xyzzy/foobar",
					},
				),
			},
		},
		{
			name: "reject custom seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                            true,
								RejectUnconfinedSeccompAdjustment: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType: api.SecurityProfile_UNCONFINED,
					},
				),
			},
			fail: true,
		},
		{
			name: "allow custom seccomp policy adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                            true,
								RejectUnconfinedSeccompAdjustment: false,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.SetLinuxSeccompPolicy(
								func() *api.LinuxSeccomp {
									seccomp := rspec.LinuxSeccomp{
										DefaultAction: rspec.ActAllow,
										ListenerPath:  "/run/meshuggah-rocks.sock",
										Architectures: []rspec.Arch{},
										Flags:         []rspec.LinuxSeccompFlag{},
										Syscalls: []rspec.LinuxSyscall{{
											Names:  []string{"sched_getaffinity"},
											Action: rspec.ActNotify,
											Args:   []rspec.LinuxSeccompArg{},
										}},
									}
									return api.FromOCILinuxSeccomp(&seccomp)
								}(),
							)
						}),
					},
				},
			},
			container: []ContainerOption{
				WithContainerSeccompProfile(
					&api.SecurityProfile{
						ProfileType: api.SecurityProfile_UNCONFINED,
					},
				),
			},
		},
		{
			name: "reject Linux namespace adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                    true,
								RejectNamespaceAdjustment: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(
								&api.LinuxNamespace{
									Type: "cgroup",
									Path: "/",
								},
							)
						}),
					},
				},
			},
			fail: true,
		},
		{
			name: "allow Linux namespace adjustment",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable:                    true,
								RejectNamespaceAdjustment: false,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {
						adjust(func(a *api.ContainerAdjustment) {
							a.AddOrReplaceNamespace(
								&api.LinuxNamespace{
									Type: "cgroup",
									Path: "/",
								},
							)
						}),
					},
				},
			},
		},
		{
			name: "reject container creation if required plugin is missing",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
								RequiredPlugins: []string{
									"foo",
									"bar",
								},
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
					"01-foo":  {},
				},
			},
			fail: true,
		},
		{
			name: "allow container creation if all required plugins are present",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
								RequiredPlugins: []string{
									"foo",
									"bar",
								},
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
					"01-foo":  {},
					"02-bar":  {},
				},
			},
		},
		{
			name: "allow container creation if missing plugin is tolerated",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
								RequiredPlugins: []string{
									"foo",
									"bar",
								},
								TolerateMissingAnnotation: "tolerate-missing-plugins." + pluginhelpers.AnnotationDomain,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
					"01-foo":  {},
				},
			},
			pod: []PodOption{
				WithPodAnnotations(
					map[string]string{
						"tolerate-missing-plugins." + pluginhelpers.AnnotationDomain + "/container.ctr0": "true",
					},
				),
			},
		},
		{
			name: "reject container creation if annotated required plugin is missing",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test": {},
				},
			},
			pod: []PodOption{
				WithPodAnnotations(
					map[string]string{
						"required-plugins." + pluginhelpers.AnnotationDomain + "/container.ctr0": "[ \"xyzzy\" ]",
					},
				),
			},
			fail: true,
		},
		{
			name: "allow container creation if all annotated required plugin are present",
			testbase: &testbase{
				runtime: []RuntimeOption{
					WithNRIRuntimeOptions(
						nri.WithDefaultValidator(
							&validator.DefaultValidatorConfig{
								Enable: true,
							},
						),
					),
				},
				plugins: map[string][]PluginOption{
					"00-test":  {},
					"01-xyzzy": {},
				},
			},
			pod: []PodOption{
				WithPodAnnotations(
					map[string]string{
						"required-plugins." + pluginhelpers.AnnotationDomain + "/container.ctr0": "[ \"xyzzy\", \"test\" ]",
					},
				),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sut, evt = tc.Setup(t)
			sut.Start(WithWaitForPluginsToStart())

			pod := sut.NewPod(
				append(
					[]PodOption{
						WithPodRandomFill(),
						WithPodName("pod0"),
					},
					tc.pod...,
				)...,
			)

			ctr := sut.NewContainer(
				append(
					[]ContainerOption{
						WithContainerRandomFill(),
						WithContainerPod(pod),
						WithContainerName("ctr0"),
					},
					tc.container...,
				)...,
			)

			adjust, _, err := sut.CreateContainer(pod, ctr)
			if tc.fail {
				require.Nil(t, adjust, "CreateContainer should fail")
				require.Error(t, err, "CreateContainer should fail")
			} else {
				require.NotNil(t, adjust, "CreateContainer should succeed")
				require.NoError(t, err, "CreateContainer should succeed")
			}

			require.NoError(t, sut.Stop(WithWaitForPluginsToClose()), "test shutdown")
			evt.Stop()
		})
	}
}

func TestPluginInstanceUpdate(t *testing.T) {
	var (
		sut *Suite
		evt *EventCollector
		tc  = &testbase{
			plugins: map[string][]PluginOption{
				"00-test": {},
			},
		}
	)

	sut, evt = tc.Setup(t)
	sut.Start(WithWaitForPluginsToStart())

	p := NewPlugin(t, sut.Dir(), evt.Channel(),
		WithPluginIndex("00"),
		WithPluginName("test"),
	)

	p.Start()
	_, started := evt.Search(
		EventOccurred(PluginSynchronized(p.ID(), nil, nil)),
		UntilTimeout(1*time.Second),
	)
	require.True(t, started, "start new plugin instance")

	_, shutdown := evt.Search(
		EventOccurred(PluginShutdown("00-test", api.ShutdownByOtherInstance)),
		UntilTimeout(1*time.Second),
	)
	require.True(t, shutdown, "old instance shut down by new instance")

	sut.Stop()
	evt.Stop()
}

func protoDiff(a, b proto.Message) string {
	return cmp.Diff(a, b, protocmp.Transform())
}

func protoEqual(a, b proto.Message) bool {
	return cmp.Equal(a, b, cmpopts.EquateEmpty(), protocmp.Transform())
}
