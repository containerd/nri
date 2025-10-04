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
	"os"
	"path/filepath"
	"time"

	nri "github.com/containerd/nri/pkg/adaptation"
	builtinstub "github.com/containerd/nri/pkg/adaptation/builtin"
	v1alpha1stub "github.com/containerd/nri/pkg/stub/v1alpha1"
	stub "github.com/containerd/nri/pkg/stub/v1beta1"

	"github.com/containerd/nri/pkg/api/convert"
	"github.com/containerd/nri/pkg/api/v1alpha1"
	api "github.com/containerd/nri/pkg/api/v1beta1"

	"testing"

	faker "github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
)

type EventKind string

const (
	Started     EventKind = "Started"
	Failed      EventKind = "Failed"
	Closed      EventKind = "Closed"
	Stopped     EventKind = "Stopped"
	Shutdown    EventKind = "Shutdown"
	Synchronize EventKind = "Synchronize"
	Update      EventKind = "Update"
	Timeout     EventKind = "Timeout"
	Marker      EventKind = "Marker"
)

type Event interface {
	Kind() EventKind
	String() string
	Match(Event) bool
}

type EventCollector struct {
	stopCh  chan struct{}
	waitCh  chan chan Event
	eventCh chan Event
	events  []Event
}

type EventWaitHandler func(Event) bool

func StartEventCollector() *EventCollector {
	c := &EventCollector{
		stopCh:  make(chan struct{}),
		waitCh:  make(chan chan Event, 1),
		eventCh: make(chan Event, 1),
	}

	go c.collect()

	return c
}

func (c *EventCollector) collect() {
	var (
		waiters = map[chan Event]struct{}{}
		del     = func(w chan Event) {
			delete(waiters, w)
			close(w)
		}
		add = func(w chan Event) {
			waiters[w] = struct{}{}
			for _, e := range c.events {
				select {
				case dw := <-c.waitCh:
					del(dw)
					if dw == w {
						return
					}
				case w <- e:
				}
			}
		}
		stop = func() {
			for w := range waiters {
				close(w)
			}
		}
	)

	for {
		select {
		case <-c.stopCh:
			stop()
			return

		case w := <-c.waitCh:
			if _, ok := waiters[w]; !ok {
				add(w)
			} else {
				del(w)
			}

		case e := <-c.eventCh:
			c.events = append(c.events, e)
			for w := range waiters {
				select {
				case w <- e:
				case w := <-c.waitCh:
					if _, ok := waiters[w]; !ok {
						add(w)
					} else {
						del(w)
					}
				}
			}
		}
	}
}

func (c *EventCollector) Stop() {
	close(c.stopCh)
}

func (c *EventCollector) Channel() chan Event {
	return c.eventCh
}

func (c *EventCollector) Emit(e Event) {
	c.eventCh <- e
}

func (c *EventCollector) Search(handler EventWaitHandler, end Event) (Event, bool) {
	var (
		w     = make(chan Event, 1)
		start = func() {
			c.waitCh <- w
		}
		stop = func() {
			for {
				select {
				case <-w:
				default:
					c.waitCh <- w
					//revive:disable-next-line:empty-block
					for range w {
					}
					return
				}
			}
		}

		timeout <-chan time.Time
		until   Event
	)

	switch e := end.(type) {
	case *TimeoutEvent:
		timeout = e.Start()
	default:
		until = end
	}

	start()

	for {
		select {
		case <-timeout:
			stop()
			return nil, false

		case e := <-w:
			if e == nil {
				return nil, false
			}
			if until != nil && e.Match(until) {
				stop()
				return nil, false
			}
			if handler(e) {
				stop()
				return e, true
			}
		}
	}
}

type TimeoutEvent struct {
	timeout time.Duration
	started time.Time
	ch      <-chan time.Time
}

func UntilTimeout(d time.Duration) *TimeoutEvent {
	return &TimeoutEvent{
		timeout: d,
	}
}

func (e *TimeoutEvent) Kind() EventKind {
	return Timeout
}

func (e *TimeoutEvent) String() string {
	return fmt.Sprintf("WaitTimeout:%s+%s", e.started.String(), e.timeout.String())
}

func (e *TimeoutEvent) Match(oe Event) bool {
	if o, ok := oe.(*TimeoutEvent); ok {
		return o == e
	}
	return false
}

func (e *TimeoutEvent) Start() <-chan time.Time {
	if e.ch != nil {
		return e.ch
	}

	e.started = time.Now()
	e.ch = time.After(e.timeout)

	return e.ch
}

type MarkerEvent struct {
	marker string
}

var (
	EndMarker = NewMarkerEvent("end")
	UntilEnd  = EndMarker
)

func UntilMarker(marker string) *MarkerEvent {
	return NewMarkerEvent(marker)
}

func NewMarkerEvent(marker string) *MarkerEvent {
	return &MarkerEvent{
		marker: marker,
	}
}

func (e *MarkerEvent) Kind() EventKind {
	return Marker
}

func (e *MarkerEvent) String() string {
	return fmt.Sprintf("Marker:%s", e.marker)
}

func (e *MarkerEvent) Match(oe Event) bool {
	if o, ok := oe.(*MarkerEvent); ok {
		return o.marker == e.marker
	}
	return false
}

func EventOccurred(e Event) EventWaitHandler {
	return func(o Event) bool {
		return o.Match(e)
	}
}

func OrderedEventsOccurred(events ...Event) EventWaitHandler {
	return func(o Event) bool {
		for i := range events {
			if o.Match(events[i]) {
				events = events[i+1:]
				return len(events) == 0
			}
		}
		return false
	}
}

type Suite struct {
	t       *testing.T
	dir     string
	evt     *EventCollector
	options []RuntimeOption
	runtime *Runtime
	plugins []*Plugin
	stopCh  chan struct{}
}

type SuiteOption func(s *Suite) error

func WithPlugins(plugins ...*Plugin) SuiteOption {
	return func(s *Suite) error {
		s.plugins = append(s.plugins, plugins...)
		return nil
	}
}

func WithRuntimeOptions(options ...RuntimeOption) SuiteOption {
	return func(s *Suite) error {
		s.options = append(s.options, options...)
		return nil
	}
}

func NewSuite(t *testing.T, dir string, evt *EventCollector, options ...SuiteOption) *Suite {
	s := &Suite{
		t:      t,
		dir:    dir,
		evt:    evt,
		stopCh: make(chan struct{}),
	}

	for _, o := range options {
		require.NoError(t, o(s), "apply suite options")
	}

	builtin := []*builtinstub.BuiltinPlugin{}
	for _, p := range s.plugins {
		if p.builtin != nil {
			if s.runtime != nil {
				err := errors.New("can't set builtin plugins, runtime already created")
				require.NoError(t, err, "create runtime with builtin plugins")
			}
			builtin = append(
				builtin,
				&builtinstub.BuiltinPlugin{
					Base:     p.Name(),
					Index:    p.Index(),
					Handlers: p.builtin.stub,
				},
			)
		}
	}

	s.runtime = NewRuntime(t, dir, s.evt.Channel(),
		append(
			s.options,
			WithBuiltinPlugins(builtin...),
		)...,
	)

	return s
}

func (s *Suite) Dir() string {
	return s.dir
}

type SuiteStartAction func(*Suite) error

func WithWaitForPluginsToStart() SuiteStartAction {
	return func(s *Suite) error {
		for _, p := range s.plugins {
			_, ok := s.evt.Search(
				EventOccurred(PluginSynchronized(p.ID(), nil, nil)),
				UntilTimeout(3*time.Second),
			)
			if !ok {
				return fmt.Errorf("timeout waiting for plugin %q to start", p.ID())
			}
		}
		return nil
	}
}

func (s *Suite) Start(actions ...SuiteStartAction) error {
	s.runtime.Start()
	for _, p := range s.plugins {
		p.Start()
	}

	for _, a := range actions {
		if err := a(s); err != nil {
			return err
		}
	}

	return nil
}

type SuiteStopAction func(*Suite) error

func WithWaitForPluginsToClose() SuiteStopAction {
	return func(s *Suite) error {
		for _, p := range s.plugins {
			if !p.IsBuiltin() {
				_, ok := s.evt.Search(
					EventOccurred(PluginClosed(p.ID())),
					UntilTimeout(3*time.Second),
				)
				if !ok {
					return fmt.Errorf("timeout waiting for plugin %q to stop", p.ID())
				}
			}
		}
		return nil
	}
}

func (s *Suite) Stop(actions ...SuiteStopAction) error {
	var errs []error

	for _, p := range s.plugins {
		p.Stop()
	}

	for _, a := range actions {
		if err := a(s); err != nil {
			errs = append(errs, err)
		}
	}

	s.runtime.Stop()
	close(s.stopCh)

	return errors.Join(errs...)
}

func (s *Suite) Plugin(id string) *Plugin {
	for _, p := range s.plugins {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

func (s *Suite) BlockPluginSync() *nri.PluginSyncBlock {
	return s.runtime.BlockPluginSync()
}

func (s *Suite) NewPod(options ...PodOption) *api.PodSandbox {
	return s.runtime.NewPod(options...)
}

func (s *Suite) NewContainer(options ...ContainerOption) *api.Container {
	return s.runtime.NewContainer(options...)
}

func (s *Suite) StartUpdateStopPod(pod *api.PodSandbox) error {
	if err := s.RunPodSandbox(pod); err != nil {
		return err
	}

	if err := s.UpdatePodSandbox(pod, nil, nil); err != nil {
		return err
	}

	if err := s.PostUpdatePodSandbox(pod); err != nil {
		return err
	}

	if err := s.StopPodSandbox(pod); err != nil {
		return err
	}

	return s.RemovePodSandbox(pod)
}

func (s *Suite) StartUpdateStopPodAndContainer(pod *api.PodSandbox, ctr *api.Container) error {
	if err := s.RunPodSandbox(pod); err != nil {
		return err
	}

	if err := s.UpdatePodSandbox(pod, nil, nil); err != nil {
		return err
	}

	if err := s.PostUpdatePodSandbox(pod); err != nil {
		return err
	}

	if _, _, err := s.CreateContainer(pod, ctr); err != nil {
		return err
	}

	if err := s.PostCreateContainer(pod, ctr); err != nil {
		return err
	}

	if err := s.StartContainer(pod, ctr); err != nil {
		return err
	}

	if err := s.PostStartContainer(pod, ctr); err != nil {
		return err
	}

	if _, err := s.UpdateContainer(pod, ctr, nil); err != nil {
		return err
	}

	if err := s.PostUpdateContainer(pod, ctr); err != nil {
		return err
	}

	if _, err := s.StopContainer(pod, ctr); err != nil {
		return err
	}

	if err := s.RemoveContainer(pod, ctr); err != nil {
		return err
	}

	if err := s.StopPodSandbox(pod); err != nil {
		return err
	}

	return s.RemovePodSandbox(pod)
}

func (s *Suite) RunPodSandbox(pod *api.PodSandbox) error {
	return s.runtime.RunPodSandbox(context.Background(), pod)
}

func (s *Suite) UpdatePodSandbox(pod *api.PodSandbox, overhead, resources *api.LinuxResources) error {
	return s.runtime.UpdatePodSandbox(context.Background(), pod, overhead, resources)
}

func (s *Suite) PostUpdatePodSandbox(pod *api.PodSandbox) error {
	return s.runtime.PostUpdatePodSandbox(context.Background(), pod)
}

func (s *Suite) StopPodSandbox(pod *api.PodSandbox) error {
	return s.runtime.StopPodSandbox(context.Background(), pod)
}

func (s *Suite) RemovePodSandbox(pod *api.PodSandbox) error {
	return s.runtime.RemovePodSandbox(context.Background(), pod)
}

func (s *Suite) CreateContainer(pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	return s.runtime.CreateContainer(context.Background(), pod, ctr)
}

func (s *Suite) PostCreateContainer(pod *api.PodSandbox, ctr *api.Container) error {
	return s.runtime.PostCreateContainer(context.Background(), pod, ctr)
}

func (s *Suite) StartContainer(pod *api.PodSandbox, ctr *api.Container) error {
	return s.runtime.StartContainer(context.Background(), pod, ctr)
}

func (s *Suite) PostStartContainer(pod *api.PodSandbox, ctr *api.Container) error {
	return s.runtime.PostStartContainer(context.Background(), pod, ctr)
}

func (s *Suite) UpdateContainer(pod *api.PodSandbox, ctr *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	return s.runtime.UpdateContainer(context.Background(), pod, ctr, resources)
}

func (s *Suite) PostUpdateContainer(pod *api.PodSandbox, ctr *api.Container) error {
	return s.runtime.PostUpdateContainer(context.Background(), pod, ctr)
}

func (s *Suite) StopContainer(pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	return s.runtime.StopContainer(context.Background(), pod, ctr)
}

func (s *Suite) RemoveContainer(pod *api.PodSandbox, ctr *api.Container) error {
	return s.runtime.RemoveContainer(context.Background(), pod, ctr)
}

type RuntimeOption func(*Runtime) error

func WithRuntimeName(name string) RuntimeOption {
	return func(r *Runtime) error {
		r.name = name
		return nil
	}

}

func WithRuntimeVersion(version string) RuntimeOption {
	return func(r *Runtime) error {
		r.version = version
		return nil
	}

}

func WithPluginRegistrationTimeout(timeout time.Duration) RuntimeOption {
	return func(*Runtime) error {
		nri.SetPluginRegistrationTimeout(timeout)
		return nil
	}
}

func WithPluginRequestTimeout(timeout time.Duration) RuntimeOption {
	return func(*Runtime) error {
		nri.SetPluginRequestTimeout(timeout)
		return nil
	}
}

var (
	RuntimeStarted     = &RuntimeEvent{kind: Started}
	RuntimeFailed      = func(e error) Event { return &RuntimeEvent{kind: Failed, err: e} }
	RuntimeStopped     = &RuntimeEvent{kind: Stopped}
	RuntimeSynchronize = &RuntimeEvent{kind: Synchronize}
	RuntimeUpdate      = &RuntimeEvent{kind: Update}
)

type RuntimeEvent struct {
	kind EventKind
	err  error
}

func (e *RuntimeEvent) Kind() EventKind {
	return e.kind
}

func (e *RuntimeEvent) String() string {
	return "Runtime:" + string(e.kind)
}

func (e *RuntimeEvent) Match(oe Event) bool {
	if e == oe {
		return true
	}

	o, ok := oe.(*RuntimeEvent)
	if !ok {
		return false
	}

	if e.kind != o.kind {
		return false
	}

	if e.kind != Failed {
		return true
	}

	if o.err != nil {
		return true
	}

	return e.err == o.err || errors.Is(e.err, o.err)
}

func WithNRIRuntimeOptions(options ...nri.Option) RuntimeOption {
	return func(r *Runtime) error {
		r.options = append(r.options, options...)
		return nil
	}
}

func WithBuiltinPlugins(plugins ...*builtinstub.BuiltinPlugin) RuntimeOption {
	return func(r *Runtime) error {
		r.options = append(r.options, nri.WithBuiltinPlugins(plugins...))
		return nil
	}
}

type Runtime struct {
	t         *testing.T
	dir       string
	name      string
	version   string
	options   []nri.Option
	syncCB    nri.SyncFn
	updateCB  nri.UpdateFn
	r         *nri.Adaptation
	events    chan<- Event
	nextPodID int
	nextCtrID int
}

const (
	TestRuntimeName    = "test-runtime"
	TestRuntimeVersion = "v1.2.3"
	TestNRIPluginDir   = "nri/plugins"
	TestNRIConfigDir   = "nri/conf.d"
	TestNRISocket      = "nri.sock"
)

func NewRuntime(t *testing.T, dir string, events chan<- Event, options ...RuntimeOption) *Runtime {
	var (
		r = &Runtime{
			t:       t,
			dir:     dir,
			name:    TestRuntimeName,
			version: TestRuntimeVersion,
			events:  events,
		}
		pluginDir  = filepath.Join(r.dir, TestNRIPluginDir)
		configDir  = filepath.Join(r.dir, TestNRIConfigDir)
		socket     = filepath.Join(r.dir, TestNRISocket)
		nriOptions = []nri.Option{
			nri.WithPluginPath(pluginDir),
			nri.WithPluginConfigPath(configDir),
			nri.WithSocketPath(socket),
		}
		err error
	)

	nri.SetPluginRegistrationTimeout(nri.DefaultPluginRegistrationTimeout)
	nri.SetPluginRequestTimeout(nri.DefaultPluginRequestTimeout)

	r.syncCB = r.synchronize
	r.updateCB = r.update

	for _, o := range options {
		require.NoError(t, o(r), "apply runtime options")
	}

	require.NoError(r.t, os.MkdirAll(pluginDir, 0o755), "create plugin dir")
	require.NoError(r.t, os.MkdirAll(configDir, 0o755), "create config dir")

	r.r, err = nri.New(
		r.name,
		r.version,
		r.syncCB,
		r.updateCB,
		append(nriOptions, r.options...)...,
	)

	if err != nil {
		r.emit(RuntimeFailed(err))
	}

	return r
}

func (*Runtime) Name() string {
	return TestRuntimeName
}

func (*Runtime) Version() string {
	return TestRuntimeVersion
}

func (r *Runtime) Start() {
	if r.r != nil {
		err := r.r.Start()
		if err != nil {
			r.emit(RuntimeFailed(err))
		} else {
			r.emit(RuntimeStarted)
		}
	}
}

func (r *Runtime) Stop() {
	if r.r != nil {
		r.r.Stop()
		r.emit(RuntimeStopped)
	}
}

func (r *Runtime) emit(e Event) {
	if r.events != nil {
		r.events <- e
	}
}

func (r *Runtime) synchronize(ctx context.Context, cb nri.SyncCB) error {
	r.emit(RuntimeSynchronize)
	cb(ctx, []*api.PodSandbox{}, []*api.Container{})
	return nil
}

func (r *Runtime) update(_ context.Context, _ []*api.ContainerUpdate) ([]*api.ContainerUpdate, error) {
	r.emit(RuntimeUpdate)
	return nil, nil
}

func (r *Runtime) BlockPluginSync() *nri.PluginSyncBlock {
	return r.r.BlockPluginSync()
}

func (r *Runtime) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return r.r.RunPodSandbox(
		ctx,
		&api.RunPodSandboxRequest{
			Pod: pod,
		},
	)
}

func (r *Runtime) UpdatePodSandbox(ctx context.Context, pod *api.PodSandbox, overhead, resources *api.LinuxResources) error {
	_, err := r.r.UpdatePodSandbox(
		ctx,
		&api.UpdatePodSandboxRequest{
			Pod:                    pod,
			OverheadLinuxResources: overhead,
			LinuxResources:         resources,
		},
	)

	return err
}

func (r *Runtime) PostUpdatePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return r.r.PostUpdatePodSandbox(
		ctx,
		&api.PostUpdatePodSandboxRequest{
			Pod: pod,
		},
	)
}

func (r *Runtime) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return r.r.StopPodSandbox(
		ctx,
		&api.StopPodSandboxRequest{
			Pod: pod,
		},
	)
}

func (r *Runtime) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return r.r.RemovePodSandbox(
		ctx,
		&api.RemovePodSandboxRequest{
			Pod: pod,
		},
	)
}

func (r *Runtime) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	rpl, err := r.r.CreateContainer(
		ctx,
		&api.CreateContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return rpl.Adjust, rpl.Update, nil
}

func (r *Runtime) PostCreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	return r.r.PostCreateContainer(
		ctx,
		&api.PostCreateContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
}

func (r *Runtime) StartContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	return r.r.StartContainer(
		ctx,
		&api.StartContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
}

func (r *Runtime) PostStartContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	return r.r.PostStartContainer(
		ctx,
		&api.PostStartContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
}

func (r *Runtime) UpdateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	rpl, err := r.r.UpdateContainer(
		ctx,
		&api.UpdateContainerRequest{
			Pod:            pod,
			Container:      ctr,
			LinuxResources: resources,
		},
	)

	if err != nil {
		return nil, err
	}

	return rpl.Update, nil
}

func (r *Runtime) PostUpdateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	return r.r.PostUpdateContainer(
		ctx,
		&api.PostUpdateContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
}

func (r *Runtime) StopContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	rpl, err := r.r.StopContainer(
		ctx,
		&api.StopContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)

	if err != nil {
		return nil, err
	}

	return rpl.Update, nil
}

func (r *Runtime) RemoveContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	return r.r.RemoveContainer(
		ctx,
		&api.RemoveContainerRequest{
			Pod:       pod,
			Container: ctr,
		},
	)
}

type PodOption func(*api.PodSandbox)

func WithPodRandomFill() PodOption {
	return func(pod *api.PodSandbox) {
		faker.RecursiveDepth = 25
		faker.Struct(pod)
	}
}

func WithPodName(name string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Name = name
	}
}

func WithPodUID(uid string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Uid = uid
	}
}

func WithPodNamespace(namespace string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Namespace = namespace
	}
}

func WithPodLabels(labels map[string]string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Labels = labels
	}
}

func WithPodAnnotations(annotations map[string]string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Annotations = annotations
	}
}

func WithPodRuntimeHandler(handler string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.RuntimeHandler = handler
	}
}

func WithPodLinuxResources(overhead, resources *api.LinuxResources) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Linux.PodOverhead = overhead
		pod.Linux.PodResources = resources
		pod.Linux.Resources = resources
	}
}

func WithPodCgroupParent(cgroupParent string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Linux.CgroupParent = cgroupParent
	}
}

func WithPodCgroupsPath(cgroupsPath string) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Linux.CgroupsPath = cgroupsPath
	}
}

func WithPodLinuxNamespaces(namespaces []*api.LinuxNamespace) PodOption {
	return func(pod *api.PodSandbox) {
		pod.Linux.Namespaces = namespaces
	}
}

func (r *Runtime) NewPod(options ...PodOption) *api.PodSandbox {
	pod := &api.PodSandbox{}
	for _, o := range options {
		o(pod)
	}

	pod.Id = fmt.Sprintf("pod-%d", r.nextPodID)
	r.nextPodID++

	return pod
}

type ContainerOption func(*api.Container)

func WithContainerRandomFill() ContainerOption {
	return func(ctr *api.Container) {
		faker.RecursiveDepth = 25
		faker.Struct(ctr)
		ctr.State = api.ContainerState_CONTAINER_UNKNOWN
	}
}

func WithContainerPodID(id string) ContainerOption {
	return func(ctr *api.Container) {
		ctr.PodSandboxId = id
	}
}

func WithContainerPod(pod *api.PodSandbox) ContainerOption {
	return func(ctr *api.Container) {
		ctr.PodSandboxId = pod.GetId()
	}
}

func WithContainerName(name string) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Name = name
	}
}

func WithContainerState(state api.ContainerState) ContainerOption {
	return func(ctr *api.Container) {
		ctr.State = state
	}
}

func WithContainerLabels(labels map[string]string) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Labels = labels
	}
}

func WithContainerAnnotations(annotations map[string]string) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Annotations = annotations
	}
}

func WithContainerArgs(args ...string) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Args = args
	}
}

func WithContainerEnv(env map[string]string) ContainerOption {
	return func(ctr *api.Container) {
		for k, v := range env {
			ctr.Env = append(ctr.Env, k+"="+v)
		}
	}
}

func WithContainerMounts(mounts ...*api.Mount) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Mounts = mounts
	}
}

func WithContainerOCIHooks(hooks *api.Hooks) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Hooks = hooks
	}
}

func WithContainerLinuxResources(resources *api.LinuxResources) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.Resources = resources
	}
}

func WithContaineOomScoreAdj(adj int32) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.OomScoreAdj = api.Int(adj)
	}
}

func WithContinerCgroupsPath(cgroupsPath string) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.CgroupsPath = cgroupsPath
	}
}

func WithContainerLinuxNamespaces(namespaces []*api.LinuxNamespace) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.Namespaces = namespaces
	}
}

func WithContainerLinuxDevices(devices []*api.LinuxDevice) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.Devices = devices
	}
}

func WithContainerLinuxIOPriority(ioPrio *api.LinuxIOPriority) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.IoPriority = ioPrio
	}
}

func WithContainerSeccompProfile(profile *api.SecurityProfile) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.SeccompProfile = profile
	}
}

func WithContainerSeccompPolicy(policy *api.LinuxSeccomp) ContainerOption {
	return func(ctr *api.Container) {
		if ctr.Linux == nil {
			ctr.Linux = &api.LinuxContainer{}
		}
		ctr.Linux.SeccompPolicy = policy
	}
}

func WithContainerPid(pid uint32) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Pid = pid
	}
}

func WithContainerPOSIXRlimits(rlimits []*api.POSIXRlimit) ContainerOption {
	return func(ctr *api.Container) {
		ctr.Rlimits = rlimits
	}
}

func WithContainerCDIDevices(devices []*api.CDIDevice) ContainerOption {
	return func(ctr *api.Container) {
		ctr.CDIDevices = devices
	}
}

func WithContainerUser(user *api.User) ContainerOption {
	return func(ctr *api.Container) {
		ctr.User = user
	}
}

func (r *Runtime) NewContainer(options ...ContainerOption) *api.Container {
	ctr := &api.Container{}
	for _, o := range options {
		o(ctr)
	}

	ctr.Id = fmt.Sprintf("ctr-%d", r.nextCtrID)
	r.nextCtrID++

	return ctr
}

type PluginOption func(*Plugin) error

func WithPluginName(name string) PluginOption {
	return func(p *Plugin) error {
		p.name = name
		return nil
	}
}

func WithPluginIndex(idx string) PluginOption {
	return func(p *Plugin) error {
		p.idx = idx
		return nil
	}
}

func WithHandlers(h Handlers) PluginOption {
	return func(p *Plugin) error {
		if p.v1alpha1 != nil {
			return errors.New("can't set v1beta1 handlers, plugin is set to v1alpha1")
		}
		if p.builtin != nil {
			return errors.New("can't set v1beta1 handlers, plugin is set to builtin")
		}
		h := h
		p.latest = &latestPlugin{
			p: p,
			h: &h,
		}
		return nil
	}
}

type Handlers struct {
	Configure                   func(string, string, string) (api.EventMask, error)
	Synchronize                 func([]*api.PodSandbox, []*api.Container) ([]*api.ContainerUpdate, error)
	Shutdown                    func(string)
	RunPodSandbox               func(*api.PodSandbox) error
	UpdatePodSandbox            func(*api.PodSandbox, *api.LinuxResources, *api.LinuxResources) error
	PostUpdatePodSandbox        func(*api.PodSandbox) error
	StopPodSandbox              func(*api.PodSandbox) error
	RemovePodSandbox            func(*api.PodSandbox) error
	CreateContainer             func(*api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error)
	PostCreateContainer         func(*api.PodSandbox, *api.Container) error
	StartContainer              func(*api.PodSandbox, *api.Container) error
	PostStartContainer          func(*api.PodSandbox, *api.Container) error
	UpdateContainer             func(*api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error)
	PostUpdateContainer         func(*api.PodSandbox, *api.Container) error
	StopContainer               func(*api.PodSandbox, *api.Container) ([]*api.ContainerUpdate, error)
	RemoveContainer             func(*api.PodSandbox, *api.Container) error
	ValidateContainerAdjustment func(*api.ValidateContainerAdjustmentRequest) error
}

func WithV1alpha1(h V1alpha1Handlers) PluginOption {
	return func(p *Plugin) error {
		if p.latest != nil {
			return errors.New("can't set v1beta1 handlers, plugin is set to v1beta1")
		}
		if p.builtin != nil {
			return errors.New("can't set v1beta1 handlers, plugin is set to builtin")
		}
		h := h
		p.v1alpha1 = &v1alpha1Plugin{
			p: p,
			h: &h,
		}
		return nil
	}
}

type V1alpha1Handlers struct {
	Configure                   func(string, string, string) (v1alpha1.EventMask, error)
	Synchronize                 func([]*v1alpha1.PodSandbox, []*v1alpha1.Container) ([]*v1alpha1.ContainerUpdate, error)
	Shutdown                    func()
	RunPodSandbox               func(*v1alpha1.PodSandbox) error
	UpdatePodSandbox            func(*v1alpha1.PodSandbox, *v1alpha1.LinuxResources, *v1alpha1.LinuxResources) error
	StopPodSandbox              func(*v1alpha1.PodSandbox) error
	RemovePodSandbox            func(*v1alpha1.PodSandbox) error
	PostUpdatePodSandbox        func(*v1alpha1.PodSandbox) error
	CreateContainer             func(*v1alpha1.PodSandbox, *v1alpha1.Container) (*v1alpha1.ContainerAdjustment, []*v1alpha1.ContainerUpdate, error)
	StartContainer              func(*v1alpha1.PodSandbox, *v1alpha1.Container) error
	UpdateContainer             func(*v1alpha1.PodSandbox, *v1alpha1.Container, *v1alpha1.LinuxResources) ([]*v1alpha1.ContainerUpdate, error)
	StopContainer               func(*v1alpha1.PodSandbox, *v1alpha1.Container) ([]*v1alpha1.ContainerUpdate, error)
	RemoveContainer             func(*v1alpha1.PodSandbox, *v1alpha1.Container) error
	PostCreateContainer         func(*v1alpha1.PodSandbox, *v1alpha1.Container) error
	PostStartContainer          func(*v1alpha1.PodSandbox, *v1alpha1.Container) error
	PostUpdateContainer         func(*v1alpha1.PodSandbox, *v1alpha1.Container) error
	ValidateContainerAdjustment func(*v1alpha1.ValidateContainerAdjustmentRequest) error
}

func WithBuiltin(h BuiltinHandlers) PluginOption {
	return func(p *Plugin) error {
		if p.latest != nil {
			return errors.New("can't set builtin handlers, plugin is set to v1beta1")
		}
		if p.v1alpha1 != nil {
			return errors.New("can't set builtin handlers, plugin is set to v1alpha1")
		}
		h := h
		p.builtin = &builtinPlugin{
			p: p,
			h: &h,
		}
		return nil
	}
}

type BuiltinHandlers struct {
	Configure            func(*api.ConfigureRequest) (*api.ConfigureResponse, error)
	Synchronize          func(*api.SynchronizeRequest) (*api.SynchronizeResponse, error)
	RunPodSandbox        func(*api.RunPodSandboxRequest) error
	StopPodSandbox       func(*api.StopPodSandboxRequest) error
	RemovePodSandbox     func(*api.RemovePodSandboxRequest) error
	UpdatePodSandbox     func(*api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error)
	PostUpdatePodSandbox func(*api.PostUpdatePodSandboxRequest) error

	CreateContainer             func(*api.CreateContainerRequest) (*api.CreateContainerResponse, error)
	PostCreateContainer         func(*api.PostCreateContainerRequest) error
	StartContainer              func(*api.StartContainerRequest) error
	PostStartContainer          func(*api.PostStartContainerRequest) error
	UpdateContainer             func(*api.UpdateContainerRequest) (*api.UpdateContainerResponse, error)
	PostUpdateContainer         func(*api.PostUpdateContainerRequest) error
	StopContainer               func(*api.StopContainerRequest) (*api.StopContainerResponse, error)
	RemoveContainer             func(*api.RemoveContainerRequest) error
	ValidateContainerAdjustment func(*api.ValidateContainerAdjustmentRequest) error
}

type Plugin struct {
	t         *testing.T
	dir       string
	events    chan<- Event
	name      string
	idx       string
	subscribe []string
	latest    *latestPlugin
	v1alpha1  *v1alpha1Plugin
	builtin   *builtinPlugin
}

type latestPlugin struct {
	p    *Plugin
	h    *Handlers
	stub stub.Stub
}

type v1alpha1Plugin struct {
	p    *Plugin
	h    *V1alpha1Handlers
	stub v1alpha1stub.Stub
}

type builtinPlugin struct {
	p    *Plugin
	h    *BuiltinHandlers
	stub builtinstub.BuiltinHandlers
}

func NewPlugin(t *testing.T, dir string, events chan<- Event, options ...PluginOption) *Plugin {
	var (
		p = &Plugin{
			t:      t,
			dir:    dir,
			events: events,
		}
		err error
	)

	for _, o := range options {
		require.NoError(t, o(p), "apply plugin options")
	}

	switch {
	default:
		p.latest = &latestPlugin{p: p, h: &Handlers{}}
		fallthrough
	case p.latest != nil:
		p.latest.stub, err = stub.New(
			p.latest,
			stub.WithPluginName(p.name),
			stub.WithPluginIdx(p.idx),
			stub.WithSocketPath(filepath.Join(p.dir, TestNRISocket)),
			stub.WithOnClose(p.onClose),
		)
		require.NoError(t, err, "create v1beta1 plugin stub")

	case p.v1alpha1 != nil:
		p.v1alpha1.stub, err = v1alpha1stub.New(
			p.v1alpha1,
			v1alpha1stub.WithPluginName(p.name),
			v1alpha1stub.WithPluginIdx(p.idx),
			v1alpha1stub.WithSocketPath(filepath.Join(p.dir, TestNRISocket)),
			v1alpha1stub.WithOnClose(p.onClose),
		)
		require.NoError(t, err, "create v1alpha1 plugin stub")

	case p.builtin != nil:
		p.builtin.stub = builtinstub.BuiltinHandlers{
			Configure:                   p.builtin.Configure,
			Synchronize:                 p.builtin.Synchronize,
			RunPodSandbox:               p.builtin.RunPodSandbox,
			UpdatePodSandbox:            p.builtin.UpdatePodSandbox,
			PostUpdatePodSandbox:        p.builtin.PostUpdatePodSandbox,
			StopPodSandbox:              p.builtin.StopPodSandbox,
			RemovePodSandbox:            p.builtin.RemovePodSandbox,
			CreateContainer:             p.builtin.CreateContainer,
			PostCreateContainer:         p.builtin.PostCreateContainer,
			StartContainer:              p.builtin.StartContainer,
			PostStartContainer:          p.builtin.PostStartContainer,
			UpdateContainer:             p.builtin.UpdateContainer,
			PostUpdateContainer:         p.builtin.PostUpdateContainer,
			StopContainer:               p.builtin.StopContainer,
			RemoveContainer:             p.builtin.RemoveContainer,
			ValidateContainerAdjustment: p.builtin.ValidateContainerAdjustment,
		}
	}

	return p
}

func (p *Plugin) ID() string {
	return p.idx + "-" + p.name
}

func (p *Plugin) Name() string {
	return p.name
}

func (p *Plugin) Index() string {
	return p.idx
}

func (p *Plugin) IsV1beta1() bool {
	return p.latest != nil
}

func (p *Plugin) IsV1alpha1() bool {
	return p.v1alpha1 != nil
}

func (p *Plugin) IsBuiltin() bool {
	return p.builtin != nil
}

func (p *Plugin) Kind() string {
	switch {
	case p.IsV1beta1():
		return "v1beta1"
	case p.IsV1alpha1():
		return "v1alpha1"
	case p.IsBuiltin():
		return "builtin"
	default:
		return "unknown"
	}
}

func (p *Plugin) Start() {
	var err error

	switch {
	case p.latest != nil:
		err = p.latest.stub.Start(context.Background())
	case p.v1alpha1 != nil:
		err = p.v1alpha1.stub.Start(context.Background())
	case p.builtin != nil:
		// Builtin plugins are started by the runtime/adaptation.
	}

	if err != nil {
		p.Emit(PluginFailed(p.ID(), err))
		return
	}

	p.Emit(PluginStarted(p.ID()))
}

func (p *Plugin) Stop() {
	switch {
	case p.latest != nil:
		p.latest.stub.Stop()
	case p.v1alpha1 != nil:
		p.v1alpha1.stub.Stop()
	case p.builtin != nil:
		// Builtin plugins are stopped by the runtime/adaptation.
	}

	p.Emit(PluginStopped(p.ID()))
}

func (p *Plugin) onClose() {
	p.Emit(PluginClosed(p.ID()))
}

func (p *Plugin) Emit(e Event) {
	if p.events != nil {
		p.events <- e
	}
}

func (p *latestPlugin) Configure(_ context.Context, cfg, runtime, version string) (api.EventMask, error) {
	p.p.Emit(
		PluginConfigured(
			p.p.ID(),
			runtime,
			version,
			p.stub.RegistrationTimeout(),
			p.stub.RequestTimeout(),
		),
	)

	if p.h.Configure != nil {
		return p.h.Configure(cfg, version, runtime)
	}

	events, err := api.ParseEventMask(p.p.subscribe...)
	require.NoError(p.p.t, err, "parse event subscription mask %v", p.p.subscribe)

	return events, nil
}

func (p *latestPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	p.p.Emit(
		PluginSynchronized(
			p.p.ID(),
			pods,
			containers,
		),
	)

	if p.h.Synchronize != nil {
		return p.h.Synchronize(pods, containers)
	}

	return nil, nil
}

func (p *latestPlugin) Shutdown(_ context.Context, reason string) {
	p.p.Emit(
		PluginShutdown(
			p.p.ID(),
			reason,
		),
	)

	if p.h.Shutdown != nil {
		p.h.Shutdown(reason)
	}
}

func (p *latestPlugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.p.Emit(
		PluginRunPodSandbox(
			p.p.ID(),
			pod,
		),
	)

	if p.h.RunPodSandbox != nil {
		return p.h.RunPodSandbox(pod)
	}

	return nil
}

func (p *latestPlugin) UpdatePodSandbox(_ context.Context, pod *api.PodSandbox, overhead, resources *api.LinuxResources) error {
	p.p.Emit(
		PluginUpdatePodSandbox(
			p.p.ID(),
			pod,
			overhead,
			resources,
		),
	)

	if p.h.UpdatePodSandbox != nil {
		return p.h.UpdatePodSandbox(pod, overhead, resources)
	}

	return nil
}

func (p *latestPlugin) PostUpdatePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.p.Emit(
		PluginPostUpdatePodSandbox(
			p.p.ID(),
			pod,
		),
	)

	if p.h.PostUpdatePodSandbox != nil {
		return p.h.PostUpdatePodSandbox(pod)
	}

	return nil
}

func (p *latestPlugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.p.Emit(
		PluginStopPodSandbox(
			p.p.ID(),
			pod,
		),
	)

	if p.h.StopPodSandbox != nil {
		return p.h.StopPodSandbox(pod)
	}

	return nil
}

func (p *latestPlugin) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	p.p.Emit(
		PluginRemovePodSandbox(
			p.p.ID(),
			pod,
		),
	)

	if p.h.RemovePodSandbox != nil {
		return p.h.RemovePodSandbox(pod)
	}

	return nil
}

func (p *latestPlugin) CreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	p.p.Emit(
		PluginCreateContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.CreateContainer != nil {
		return p.h.CreateContainer(pod, container)
	}

	return nil, nil, nil
}

func (p *latestPlugin) PostCreateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.p.Emit(
		PluginPostCreateContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.PostCreateContainer != nil {
		return p.h.PostCreateContainer(pod, container)
	}

	return nil
}

func (p *latestPlugin) StartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.p.Emit(
		PluginStartContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.StartContainer != nil {
		return p.h.StartContainer(pod, container)
	}

	return nil
}

func (p *latestPlugin) PostStartContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.p.Emit(
		PluginPostStartContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.PostStartContainer != nil {
		return p.h.PostStartContainer(pod, container)
	}

	return nil
}

func (p *latestPlugin) UpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	p.p.Emit(
		PluginUpdateContainer(
			p.p.ID(),
			pod,
			container,
			resources,
		),
	)

	if p.h.UpdateContainer != nil {
		return p.h.UpdateContainer(pod, container, resources)
	}

	return nil, nil
}

func (p *latestPlugin) PostUpdateContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.p.Emit(
		PluginPostUpdateContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.PostUpdateContainer != nil {
		return p.h.PostUpdateContainer(pod, container)
	}

	return nil
}

func (p *latestPlugin) StopContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	p.p.Emit(
		PluginStopContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.StopContainer != nil {
		return p.h.StopContainer(pod, container)
	}

	return nil, nil
}

func (p *latestPlugin) RemoveContainer(_ context.Context, pod *api.PodSandbox, container *api.Container) error {
	p.p.Emit(
		PluginRemoveContainer(
			p.p.ID(),
			pod,
			container,
		),
	)

	if p.h.RemoveContainer != nil {
		return p.h.RemoveContainer(pod, container)
	}

	return nil
}

func (p *latestPlugin) ValidateContainerAdjustment(_ context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	p.p.Emit(
		PluginValidateContainerAdjustment(
			p.p.ID(),
			req,
		),
	)

	if p.h.ValidateContainerAdjustment != nil {
		return p.h.ValidateContainerAdjustment(req)
	}

	return nil
}

func (p *v1alpha1Plugin) Configure(_ context.Context, cfg, runtime, version string) (v1alpha1.EventMask, error) {
	p.p.Emit(
		PluginConfigured(
			p.p.ID(),
			runtime,
			version,
			p.stub.RegistrationTimeout(),
			p.stub.RequestTimeout(),
		),
	)

	if p.h.Configure != nil {
		return p.h.Configure(cfg, version, runtime)
	}

	events, err := v1alpha1.ParseEventMask(p.p.subscribe...)
	require.NoError(p.p.t, err, "parse event subscription mask %v", p.p.subscribe)

	return events, nil
}

func (p *v1alpha1Plugin) Synchronize(_ context.Context, pods []*v1alpha1.PodSandbox, containers []*v1alpha1.Container) ([]*v1alpha1.ContainerUpdate, error) {
	p.p.Emit(
		PluginSynchronized(
			p.p.ID(),
			convert.PodSandboxSliceToV1beta1(pods),
			convert.ContainerSliceToV1beta1(containers),
		),
	)

	if p.h.Synchronize != nil {
		return p.h.Synchronize(pods, containers)
	}

	return nil, nil
}

func (p *v1alpha1Plugin) Shutdown(_ context.Context) {
	p.p.Emit(
		PluginShutdown(
			p.p.ID(),
			"",
		),
	)

	if p.h.Shutdown != nil {
		p.h.Shutdown()
	}
}

func (p *v1alpha1Plugin) RunPodSandbox(_ context.Context, pod *v1alpha1.PodSandbox) error {
	p.p.Emit(
		PluginRunPodSandbox(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
		),
	)

	if p.h.RunPodSandbox != nil {
		return p.h.RunPodSandbox(pod)
	}

	return nil
}

func (p *v1alpha1Plugin) UpdatePodSandbox(_ context.Context, pod *v1alpha1.PodSandbox, overhead, resources *v1alpha1.LinuxResources) error {
	p.p.Emit(
		PluginUpdatePodSandbox(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.LinuxResourcesToV1beta1(overhead),
			convert.LinuxResourcesToV1beta1(resources),
		),
	)

	if p.h.UpdatePodSandbox != nil {
		return p.h.UpdatePodSandbox(pod, overhead, resources)
	}

	return nil
}

func (p *v1alpha1Plugin) PostUpdatePodSandbox(_ context.Context, pod *v1alpha1.PodSandbox) error {
	p.p.Emit(
		PluginPostUpdatePodSandbox(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
		),
	)

	if p.h.PostUpdatePodSandbox != nil {
		return p.h.PostUpdatePodSandbox(pod)
	}

	return nil
}

func (p *v1alpha1Plugin) StopPodSandbox(_ context.Context, pod *v1alpha1.PodSandbox) error {
	p.p.Emit(
		PluginStopPodSandbox(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
		),
	)

	if p.h.StopPodSandbox != nil {
		return p.h.StopPodSandbox(pod)
	}

	return nil
}

func (p *v1alpha1Plugin) RemovePodSandbox(_ context.Context, pod *v1alpha1.PodSandbox) error {
	p.p.Emit(
		PluginRemovePodSandbox(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
		),
	)

	if p.h.RemovePodSandbox != nil {
		return p.h.RemovePodSandbox(pod)
	}

	return nil
}

func (p *v1alpha1Plugin) CreateContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) (*v1alpha1.ContainerAdjustment, []*v1alpha1.ContainerUpdate, error) {
	p.p.Emit(
		PluginCreateContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.CreateContainer != nil {
		return p.h.CreateContainer(pod, container)
	}

	return nil, nil, nil
}

func (p *v1alpha1Plugin) PostCreateContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) error {
	p.p.Emit(
		PluginPostCreateContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.PostCreateContainer != nil {
		return p.h.PostCreateContainer(pod, container)
	}

	return nil
}

func (p *v1alpha1Plugin) StartContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) error {
	p.p.Emit(
		PluginStartContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.StartContainer != nil {
		return p.h.StartContainer(pod, container)
	}

	return nil
}

func (p *v1alpha1Plugin) PostStartContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) error {
	p.p.Emit(
		PluginPostStartContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.PostStartContainer != nil {
		return p.h.PostStartContainer(pod, container)
	}

	return nil
}

func (p *v1alpha1Plugin) UpdateContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container, resources *v1alpha1.LinuxResources) ([]*v1alpha1.ContainerUpdate, error) {
	p.p.Emit(
		PluginUpdateContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
			convert.LinuxResourcesToV1beta1(resources),
		),
	)

	if p.h.UpdateContainer != nil {
		return p.h.UpdateContainer(pod, container, resources)
	}

	return nil, nil
}

func (p *v1alpha1Plugin) PostUpdateContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) error {
	p.p.Emit(
		PluginPostUpdateContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.PostUpdateContainer != nil {
		return p.h.PostUpdateContainer(pod, container)
	}

	return nil
}

func (p *v1alpha1Plugin) StopContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) ([]*v1alpha1.ContainerUpdate, error) {
	p.p.Emit(
		PluginStopContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.StopContainer != nil {
		return p.h.StopContainer(pod, container)
	}

	return nil, nil
}

func (p *v1alpha1Plugin) RemoveContainer(_ context.Context, pod *v1alpha1.PodSandbox, container *v1alpha1.Container) error {
	p.p.Emit(
		PluginRemoveContainer(
			p.p.ID(),
			convert.PodSandboxToV1beta1(pod),
			convert.ContainerToV1beta1(container),
		),
	)

	if p.h.RemoveContainer != nil {
		return p.h.RemoveContainer(pod, container)
	}

	return nil
}

func (p *v1alpha1Plugin) ValidateContainerAdjustment(_ context.Context, req *v1alpha1.ValidateContainerAdjustmentRequest) error {
	p.p.Emit(
		PluginValidateContainerAdjustment(
			p.p.ID(),
			convert.ValidateContainerAdjustmentRequestToV1beta1(req),
		),
	)

	if p.h.ValidateContainerAdjustment != nil {
		return p.h.ValidateContainerAdjustment(req)
	}

	return nil
}

func (p *builtinPlugin) Configure(_ context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	p.p.Emit(
		PluginConfigured(
			p.p.ID(),
			req.RuntimeName,
			req.RuntimeVersion,
			time.Duration(req.RegistrationTimeout*int64(time.Millisecond)),
			time.Duration(req.RequestTimeout*int64(time.Millisecond)),
		),
	)

	if p.h.Configure != nil {
		return p.h.Configure(req)
	}

	events, err := api.ParseEventMask(p.p.subscribe...)
	require.NoError(p.p.t, err, "parse event subscription mask %v", p.p.subscribe)

	return &api.ConfigureResponse{
		Events: int32(events),
	}, nil
}

func (p *builtinPlugin) Synchronize(_ context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	p.p.Emit(
		PluginSynchronized(
			p.p.ID(),
			req.Pods,
			req.Containers,
		),
	)

	if p.h.Synchronize != nil {
		return p.h.Synchronize(req)
	}

	return &api.SynchronizeResponse{}, nil
}

func (p *builtinPlugin) RunPodSandbox(_ context.Context, req *api.RunPodSandboxRequest) error {
	p.p.Emit(
		PluginRunPodSandbox(
			p.p.ID(),
			req.Pod,
		),
	)

	if p.h.RunPodSandbox != nil {
		return p.h.RunPodSandbox(req)
	}

	return nil
}

func (p *builtinPlugin) UpdatePodSandbox(_ context.Context, req *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	p.p.Emit(
		PluginUpdatePodSandbox(
			p.p.ID(),
			req.Pod,
			req.OverheadLinuxResources,
			req.LinuxResources,
		),
	)

	if p.h.UpdatePodSandbox != nil {
		return p.h.UpdatePodSandbox(req)
	}

	return &api.UpdatePodSandboxResponse{}, nil
}

func (p *builtinPlugin) PostUpdatePodSandbox(_ context.Context, req *api.PostUpdatePodSandboxRequest) error {
	p.p.Emit(
		PluginPostUpdatePodSandbox(
			p.p.ID(),
			req.Pod,
		),
	)

	if p.h.PostUpdatePodSandbox != nil {
		return p.h.PostUpdatePodSandbox(req)
	}

	return nil
}

func (p *builtinPlugin) StopPodSandbox(_ context.Context, req *api.StopPodSandboxRequest) error {
	p.p.Emit(
		PluginStopPodSandbox(
			p.p.ID(),
			req.Pod,
		),
	)

	if p.h.StopPodSandbox != nil {
		return p.h.StopPodSandbox(req)
	}

	return nil
}

func (p *builtinPlugin) RemovePodSandbox(_ context.Context, req *api.RemovePodSandboxRequest) error {
	p.p.Emit(
		PluginRemovePodSandbox(
			p.p.ID(),
			req.Pod,
		),
	)

	if p.h.RemovePodSandbox != nil {
		return p.h.RemovePodSandbox(req)
	}

	return nil
}

func (p *builtinPlugin) CreateContainer(_ context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	p.p.Emit(
		PluginCreateContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.CreateContainer != nil {
		return p.h.CreateContainer(req)
	}

	return &api.CreateContainerResponse{}, nil
}

func (p *builtinPlugin) PostCreateContainer(_ context.Context, req *api.PostCreateContainerRequest) error {
	p.p.Emit(
		PluginPostCreateContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.PostCreateContainer != nil {
		return p.h.PostCreateContainer(req)
	}

	return nil
}

func (p *builtinPlugin) StartContainer(_ context.Context, req *api.StartContainerRequest) error {
	p.p.Emit(
		PluginStartContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.StartContainer != nil {
		return p.h.StartContainer(req)
	}

	return nil
}

func (p *builtinPlugin) PostStartContainer(_ context.Context, req *api.PostStartContainerRequest) error {
	p.p.Emit(
		PluginPostStartContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.PostStartContainer != nil {
		return p.h.PostStartContainer(req)
	}

	return nil
}

func (p *builtinPlugin) UpdateContainer(_ context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	p.p.Emit(
		PluginUpdateContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
			req.LinuxResources,
		),
	)

	if p.h.UpdateContainer != nil {
		return p.h.UpdateContainer(req)
	}

	return &api.UpdateContainerResponse{}, nil
}

func (p *builtinPlugin) PostUpdateContainer(_ context.Context, req *api.PostUpdateContainerRequest) error {
	p.p.Emit(
		PluginPostUpdateContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.PostUpdateContainer != nil {
		return p.h.PostUpdateContainer(req)
	}

	return nil
}

func (p *builtinPlugin) StopContainer(_ context.Context, req *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	p.p.Emit(
		PluginStopContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.StopContainer != nil {
		return p.h.StopContainer(req)
	}

	return &api.StopContainerResponse{}, nil
}

func (p *builtinPlugin) RemoveContainer(_ context.Context, req *api.RemoveContainerRequest) error {
	p.p.Emit(
		PluginRemoveContainer(
			p.p.ID(),
			req.Pod,
			req.Container,
		),
	)

	if p.h.RemoveContainer != nil {
		return p.h.RemoveContainer(req)
	}

	return nil
}

func (p *builtinPlugin) ValidateContainerAdjustment(_ context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	p.p.Emit(
		PluginValidateContainerAdjustment(
			p.p.ID(),
			req,
		),
	)

	if p.h.ValidateContainerAdjustment != nil {
		return p.h.ValidateContainerAdjustment(req)
	}

	return nil
}

type PluginEvent struct {
	kind                EventKind
	plugin              string
	err                 error
	reason              string
	runtimeName         string
	runtimeVersion      string
	registrationTimeout time.Duration
	requestTimeout      time.Duration
	pods                []*api.PodSandbox
	containers          []*api.Container
	pod                 *api.PodSandbox
	container           *api.Container
	overhead            *api.LinuxResources
	resources           *api.LinuxResources
	validate            *api.ValidateContainerAdjustmentRequest
}

const (
	Configured                  EventKind = "Configured"
	RunPodSandbox               EventKind = "RunPodSandbox"
	UpdatePodSandbox            EventKind = "UpdatePodSandbox"
	PostUpdatePodSandbox        EventKind = "PostUpdatePodSandbox"
	StopPodSandbox              EventKind = "StopPodSandbox"
	RemovePodSandbox            EventKind = "RemovePodSandbox"
	CreateContainer             EventKind = "CreateContainer"
	PostCreateContainer         EventKind = "PostCreateContainer"
	StartContainer              EventKind = "StartContainer"
	PostStartContainer          EventKind = "PostStartContainer"
	UpdateContainer             EventKind = "UpdateContainer"
	PostUpdateContainer         EventKind = "PostUpdateContainer"
	StopContainer               EventKind = "StopContainer"
	RemoveContainer             EventKind = "RemoveContainer"
	ValidateContainerAdjustment EventKind = "ValidateContainerAdjustment"
)

func (e *PluginEvent) Kind() EventKind {
	return EventKind(fmt.Sprintf("%s:%s", e.plugin, e.kind))
}

func (e *PluginEvent) String() string {
	s := fmt.Sprintf("%s:%s", e.plugin, e.kind)

	switch e.kind {
	case Failed:
		if e.err != nil {
			s += fmt.Sprintf(" %v", e.err)
		}
	}

	return s
}

func (e *PluginEvent) Match(oe Event) bool {
	o, ok := oe.(*PluginEvent)
	if !ok {
		return false
	}
	if e.kind != o.kind || e.plugin != o.plugin {
		return false
	}

	switch e.kind {
	case Failed:
		if errors.Is(e.err, o.err) || errors.Is(o.err, e.err) {
			return true
		}
		if e.err != nil && o.err != nil {
			return fmt.Sprintf("%v", e.err) == fmt.Sprintf("%v", o.err)
		}

	case Configured:
		switch {
		case e.runtimeName != "" && o.runtimeName != "":
			return e.runtimeName == o.runtimeName
		case e.runtimeVersion != "" && o.runtimeVersion != "":
			return e.runtimeVersion == o.runtimeVersion
		case e.registrationTimeout != 0 && o.registrationTimeout != 0:
			return e.registrationTimeout == o.registrationTimeout
		case e.requestTimeout != 0 && o.requestTimeout != 0:
			return e.requestTimeout == o.requestTimeout
		}
	}

	return true
}

func PluginStarted(plugin string) *PluginEvent {
	return &PluginEvent{
		kind:   Started,
		plugin: plugin,
	}
}

func PluginStopped(plugin string) *PluginEvent {
	return &PluginEvent{
		kind:   Stopped,
		plugin: plugin,
	}
}

func PluginFailed(plugin string, err error) *PluginEvent {
	return &PluginEvent{
		kind:   Failed,
		plugin: plugin,
		err:    err,
	}
}

func PluginClosed(plugin string) *PluginEvent {
	return &PluginEvent{
		kind:   Closed,
		plugin: plugin,
	}
}
func PluginShutdown(plugin, reason string) *PluginEvent {
	return &PluginEvent{
		kind:   Shutdown,
		plugin: plugin,
		reason: reason,
	}
}

func PluginConfigured(plugin string, runtime, version string, registration, request time.Duration) *PluginEvent {
	return &PluginEvent{
		kind:                Configured,
		plugin:              plugin,
		runtimeName:         runtime,
		runtimeVersion:      version,
		registrationTimeout: registration,
		requestTimeout:      request,
	}
}

func PluginSynchronized(plugin string, pods []*api.PodSandbox, containers []*api.Container) *PluginEvent {
	return &PluginEvent{
		kind:       Synchronize,
		plugin:     plugin,
		pods:       pods,
		containers: containers,
	}
}

func PluginRunPodSandbox(plugin string, pod *api.PodSandbox) *PluginEvent {
	return &PluginEvent{
		kind:   RunPodSandbox,
		plugin: plugin,
		pod:    pod,
	}
}

func PluginUpdatePodSandbox(plugin string, pod *api.PodSandbox, overhead, resources *api.LinuxResources) *PluginEvent {
	return &PluginEvent{
		kind:      UpdatePodSandbox,
		plugin:    plugin,
		pod:       pod,
		overhead:  overhead,
		resources: resources,
	}
}

func PluginPostUpdatePodSandbox(plugin string, pod *api.PodSandbox) *PluginEvent {
	return &PluginEvent{
		kind:   PostUpdatePodSandbox,
		plugin: plugin,
		pod:    pod,
	}
}

func PluginStopPodSandbox(plugin string, pod *api.PodSandbox) *PluginEvent {
	return &PluginEvent{
		kind:   StopPodSandbox,
		plugin: plugin,
		pod:    pod,
	}
}

func PluginRemovePodSandbox(plugin string, pod *api.PodSandbox) *PluginEvent {
	return &PluginEvent{
		kind:   RemovePodSandbox,
		plugin: plugin,
		pod:    pod,
	}
}

func PluginCreateContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      CreateContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginPostCreateContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      PostCreateContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginStartContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      StartContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginPostStartContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      PostStartContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginUpdateContainer(plugin string, pod *api.PodSandbox, container *api.Container, resources *api.LinuxResources) *PluginEvent {
	return &PluginEvent{
		kind:      UpdateContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
		resources: resources,
	}
}

func PluginPostUpdateContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      PostUpdateContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginStopContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      StopContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginRemoveContainer(plugin string, pod *api.PodSandbox, container *api.Container) *PluginEvent {
	return &PluginEvent{
		kind:      RemoveContainer,
		plugin:    plugin,
		pod:       pod,
		container: container,
	}
}

func PluginValidateContainerAdjustment(plugin string, req *api.ValidateContainerAdjustmentRequest) *PluginEvent {
	return &PluginEvent{
		kind:     ValidateContainerAdjustment,
		plugin:   plugin,
		validate: req,
	}
}
