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
	"sort"
	"sync"
	"testing"
	"time"

	nri "github.com/containerd/nri/pkg/adaptation"
	api "github.com/containerd/nri/pkg/api/v1beta1"
	"github.com/sirupsen/logrus"

	stub "github.com/containerd/nri/pkg/stub/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRuntime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NRI Runtime")
}

const (
	startupTimeout        = 2 * time.Second
	defaultRuntimeName    = "default-runtime-name"
	defaultRuntimeVersion = "0.1.2"
)

// A test suite consist of a runtime and a set of plugins.
type Suite struct {
	dir     string        // directory to create for test
	runtime *mockRuntime  // runtime instance for test
	plugins []*mockPlugin // plugin instances for test
	byName  map[string]*mockPlugin
}

// SuiteOption can be applied to a suite.
type SuiteOption func(s *Suite) error

// Prepare test suite, creating test directory.
func (s *Suite) Prepare(runtime *mockRuntime, plugins ...*mockPlugin) string {
	var (
		dir string
		etc string
	)

	logrus.SetLevel(logrus.ErrorLevel)

	dir = GinkgoT().TempDir()
	etc = filepath.Join(dir, "etc", "nri")

	Expect(os.MkdirAll(etc, 0o755)).To(Succeed())

	if runtime.name == "" {
		runtime.name = defaultRuntimeName
	}
	if runtime.version == "" {
		runtime.version = defaultRuntimeVersion
	}

	s.dir = dir
	s.runtime = runtime
	s.plugins = plugins

	if s.byName == nil {
		s.byName = make(map[string]*mockPlugin)
	}

	return dir
}

// Dir returns the suite's temporary test directory.
func (s *Suite) Dir() string {
	return s.dir
}

// Startup starts up the test suite.
func (s *Suite) Startup() {
	plugins := s.plugins
	s.plugins = nil
	s.StartRuntime()
	s.StartPlugins(plugins...)
	s.WaitForPluginsToSync(plugins...)
}

// StartRuntime starts the suite runtime.
func (s *Suite) StartRuntime() {
	Expect(s.runtime.Start(s.dir)).To(Succeed())
}

// StartPlugins starts the suite plugins.
func (s *Suite) StartPlugins(plugins ...*mockPlugin) {
	for _, plugin := range plugins {
		s.plugins = append(s.plugins, plugin)
		s.byName[plugin.FullName()] = plugin
		Expect(plugin.Start(s.dir)).To(Succeed())
	}
}

// WaitForPluginsToSync waits for the given plugins to get synchronized.
func (s *Suite) WaitForPluginsToSync(plugins ...*mockPlugin) {
	timeout := time.After(startupTimeout)
	for _, plugin := range plugins {
		Expect(plugin.Wait(PluginSynchronized, timeout)).To(Succeed())
	}
	s.runtime.runtime.BlockPluginSync().Unblock() // ensure plugins are fully registered
}

// Cleanup the test suite.
func (s *Suite) Cleanup() {
	s.runtime.Stop()
	// TODO(klihub):
	for _, plugin := range s.plugins {
		plugin.Stop()
	}
	Expect(os.RemoveAll(s.dir)).To(Succeed())
}

// Plugin returns a plugin started by StartPlugins by full plugin name.
func (s *Suite) plugin(fullName string) *mockPlugin {
	return s.byName[fullName]
}

// ------------------------------------

func Log(format string, args ...interface{}) {
	GinkgoWriter.Printf(format+"\n", args...)
}

type mockRuntime struct {
	name    string
	version string
	options []nri.Option
	runtime *nri.Adaptation
	pods    map[string]*api.PodSandbox
	ctrs    map[string]*api.Container

	updateFn nri.UpdateFn
}

func (m *mockRuntime) Start(dir string) error {
	var (
		options = []nri.Option{
			nri.WithPluginPath(filepath.Join(dir, "opt", "nri", "plugins")),
			nri.WithPluginConfigPath(filepath.Join(dir, "etc", "nri", "conf.d")),
			nri.WithSocketPath(filepath.Join(dir, "nri.sock")),
		}
		err error
	)

	if m.runtime != nil {
		return errors.New("mock runtime already started")
	}

	options = append(options, m.options...)
	m.runtime, err = nri.New(m.name, m.version, m.synchronize, m.update, options...)
	if err != nil {
		return err
	}

	if m.pods == nil {
		m.pods = make(map[string]*api.PodSandbox)
	}
	if m.ctrs == nil {
		m.ctrs = make(map[string]*api.Container)
	}

	if m.updateFn == nil {
		m.updateFn = func(context.Context, []*api.ContainerUpdate) ([]*api.ContainerUpdate, error) {
			return nil, nil
		}
	}

	return m.runtime.Start()
}

func (m *mockRuntime) Stop() {
	if m.runtime != nil {
		m.runtime.Stop()
		m.runtime = nil
	}
}

func (m *mockRuntime) synchronize(ctx context.Context, cb nri.SyncCB) error {
	var (
		pods []*api.PodSandbox
		ctrs []*api.Container
		ids  []string
	)

	for id := range m.pods {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		pods = append(pods, m.pods[id])
	}

	ids = nil
	for id := range m.ctrs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ctrs = append(ctrs, m.ctrs[id])
	}

	_, err := cb(ctx, pods, ctrs)
	return err
}

func (m *mockRuntime) RunPodSandbox(ctx context.Context, req *api.RunPodSandboxRequest) error {
	b := m.runtime.BlockPluginSync()
	defer b.Unblock()
	return m.runtime.RunPodSandbox(ctx, req)
}

func (m *mockRuntime) UpdatePodSandbox(ctx context.Context, req *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	b := m.runtime.BlockPluginSync()
	defer b.Unblock()
	return m.runtime.UpdatePodSandbox(ctx, req)
}

func (m *mockRuntime) CreateContainer(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	b := m.runtime.BlockPluginSync()
	defer b.Unblock()
	return m.runtime.CreateContainer(ctx, req)
}

func (m *mockRuntime) UpdateContainer(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	b := m.runtime.BlockPluginSync()
	defer b.Unblock()
	return m.runtime.UpdateContainer(ctx, req)
}

func (m *mockRuntime) startStopPodAndContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	err := m.RunPodSandbox(ctx, &api.RunPodSandboxRequest{
		Pod: pod,
	})
	if err != nil {
		return err
	}

	_, err = m.UpdatePodSandbox(ctx, &api.UpdatePodSandboxRequest{
		Pod:                    pod,
		OverheadLinuxResources: &api.LinuxResources{},
		LinuxResources:         &api.LinuxResources{},
	})
	if err != nil {
		return err
	}

	err = m.runtime.PostUpdatePodSandbox(ctx, &api.PostUpdatePodSandboxRequest{
		Pod: pod,
	})
	if err != nil {
		return err
	}

	_, err = m.CreateContainer(ctx, &api.CreateContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	err = m.runtime.PostCreateContainer(ctx, &api.PostCreateContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	err = m.runtime.StartContainer(ctx, &api.StartContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	err = m.runtime.PostStartContainer(ctx, &api.PostStartContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	_, err = m.UpdateContainer(ctx, &api.UpdateContainerRequest{
		Pod:            pod,
		Container:      ctr,
		LinuxResources: &api.LinuxResources{},
	})
	if err != nil {
		return err
	}

	err = m.runtime.PostUpdateContainer(ctx, &api.PostUpdateContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	_, err = m.runtime.StopContainer(ctx, &api.StopContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	err = m.runtime.RemoveContainer(ctx, &api.RemoveContainerRequest{
		Pod:       pod,
		Container: ctr,
	})
	if err != nil {
		return err
	}

	err = m.runtime.StopPodSandbox(ctx, &api.StopPodSandboxRequest{
		Pod: pod,
	})
	if err != nil {
		return err
	}

	err = m.runtime.RemovePodSandbox(ctx, &api.RemovePodSandboxRequest{
		Pod: pod,
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *mockRuntime) update(ctx context.Context, updates []*nri.ContainerUpdate) ([]*nri.ContainerUpdate, error) {
	return m.updateFn(ctx, updates)
}

type mockPlugin struct {
	name string
	idx  string
	stub stub.Stub
	mask stub.EventMask

	runtime string
	version string

	q    *EventQ
	pods map[string]*api.PodSandbox
	ctrs map[string]*api.Container

	runPodSandbox        func(*mockPlugin, *api.PodSandbox) error
	updatePodSandbox     func(*mockPlugin, *api.PodSandbox, *api.LinuxResources, *api.LinuxResources) error
	postUpdatePodSandbox func(*mockPlugin, *api.PodSandbox) error
	stopPodSandbox       func(*mockPlugin, *api.PodSandbox) error
	removePodSandbox     func(*mockPlugin, *api.PodSandbox) error
	createContainer      func(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error)
	postCreateContainer  func(*mockPlugin, *api.PodSandbox, *api.Container) error
	startContainer       func(*mockPlugin, *api.PodSandbox, *api.Container) error
	postStartContainer   func(*mockPlugin, *api.PodSandbox, *api.Container) error
	updateContainer      func(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error)
	postUpdateContainer  func(*mockPlugin, *api.PodSandbox, *api.Container) error
	stopContainer        func(*mockPlugin, *api.PodSandbox, *api.Container) ([]*api.ContainerUpdate, error)
	removeContainer      func(*mockPlugin, *api.PodSandbox, *api.Container) error
	validateAdjustment   func(*mockPlugin, *api.ValidateContainerAdjustmentRequest) error
}

var (
	_ = stub.ConfigureInterface(&mockPlugin{})
	_ = stub.SynchronizeInterface(&mockPlugin{})
	_ = stub.RunPodInterface(&mockPlugin{})
	_ = stub.UpdatePodInterface(&mockPlugin{})
	_ = stub.StopPodInterface(&mockPlugin{})
	_ = stub.RemovePodInterface(&mockPlugin{})
	_ = stub.PostUpdatePodInterface(&mockPlugin{})
	_ = stub.CreateContainerInterface(&mockPlugin{})
	_ = stub.StartContainerInterface(&mockPlugin{})
	_ = stub.UpdateContainerInterface(&mockPlugin{})
	_ = stub.StopContainerInterface(&mockPlugin{})
	_ = stub.RemoveContainerInterface(&mockPlugin{})
	_ = stub.PostCreateContainerInterface(&mockPlugin{})
	_ = stub.PostStartContainerInterface(&mockPlugin{})
	_ = stub.PostUpdateContainerInterface(&mockPlugin{})
)

func (m *mockPlugin) Log(format string, args ...interface{}) {
	Log("* [plugin %s-%s] "+format, append([]interface{}{m.idx, m.name}, args...)...)
}

func (m *mockPlugin) SetFallbackName(name string, idx int) {
	if m.name == "" {
		m.name = name
	}
	if m.idx == "" {
		m.idx = fmt.Sprintf("%02d", idx)
	}
}

func (m *mockPlugin) Wait(e *Event, deadline <-chan time.Time) error {
	_, err := m.q.Wait(e, deadline)
	return err
}

func (m *mockPlugin) Events() []*Event {
	return m.q.Events()
}

func (m *mockPlugin) EventQ() *EventQ {
	return m.q
}

func (m *mockPlugin) Init(dir string) error {
	var err error

	if m.stub != nil {
		return fmt.Errorf("plugin %s-%s already initialized", m.idx, m.name)
	}

	if m.name == "" {
		m.name = "mock-plugin"
	}
	if m.idx == "" {
		m.idx = "00"
	}

	m.q = &EventQ{}

	m.Log("Init()...")

	m.stub, err = stub.New(m,
		stub.WithPluginName(m.name),
		stub.WithPluginIdx(m.idx),
		stub.WithSocketPath(filepath.Join(dir, "nri.sock")),
		stub.WithOnClose(m.onClose),
	)
	if err != nil {
		m.q.Add(PluginCreationError)
		return err
	}

	m.pods = make(map[string]*api.PodSandbox)
	m.ctrs = make(map[string]*api.Container)

	if m.runPodSandbox == nil {
		m.runPodSandbox = nopPodEvent
	}
	if m.updatePodSandbox == nil {
		m.updatePodSandbox = nopUpdatePodSandbox
	}
	if m.postUpdatePodSandbox == nil {
		m.postUpdatePodSandbox = nopPodEvent
	}
	if m.stopPodSandbox == nil {
		m.stopPodSandbox = nopPodEvent
	}
	if m.removePodSandbox == nil {
		m.removePodSandbox = nopPodEvent
	}
	if m.createContainer == nil {
		m.createContainer = nopCreateContainer
	}
	if m.postCreateContainer == nil {
		m.postCreateContainer = nopContainerEvent
	}
	if m.startContainer == nil {
		m.startContainer = nopContainerEvent
	}
	if m.postStartContainer == nil {
		m.postStartContainer = nopContainerEvent
	}
	if m.updateContainer == nil {
		m.updateContainer = nopUpdateContainer
	}
	if m.postUpdateContainer == nil {
		m.postUpdateContainer = nopContainerEvent
	}
	if m.stopContainer == nil {
		m.stopContainer = nopStopContainer
	}
	if m.removeContainer == nil {
		m.removeContainer = nopContainerEvent
	}

	return nil
}

func (m *mockPlugin) Start(dir string) error {
	if m.stub == nil {
		if err := m.Init(dir); err != nil {
			return err
		}
	}

	if err := m.stub.Start(context.Background()); err != nil {
		m.q.Add(PluginStartupError)
		return err
	}

	return nil
}

func (m *mockPlugin) Stop() {
	if m.stub != nil {
		m.stub.Stop()
		m.stub.Wait()
	}
	m.q.Add(PluginStopped)
}

func (m *mockPlugin) FullName() string {
	return m.idx + "-" + m.name
}

func (m *mockPlugin) RuntimeName() string {
	return m.runtime
}

func (m *mockPlugin) RuntimeVersion() string {
	return m.version
}

func (m *mockPlugin) onClose() {
	if m.stub != nil {
		m.stub.Stop()
		m.stub.Wait()
	}

	if m.q != nil {
		m.q.Add(PluginDisconnected)
	}
}

func (m *mockPlugin) Configure(_ context.Context, _, runtime, version string) (stub.EventMask, error) {
	m.q.Add(PluginConfigured)

	m.runtime = runtime
	m.version = version

	events := m.mask
	if m.validateAdjustment == nil {
		events.Clear(api.Event_VALIDATE_CONTAINER_ADJUSTMENT)
	}

	return events, nil
}

func (m *mockPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, ctrs []*api.Container) ([]*api.ContainerUpdate, error) {
	for _, pod := range pods {
		m.pods[pod.Id] = pod
	}
	for _, ctr := range ctrs {
		m.ctrs[ctr.Id] = ctr
	}

	m.q.Add(PluginSynchronized)

	return nil, nil
}

func (m *mockPlugin) Shutdown(_ context.Context) {
	m.q.Add(PluginShutdown)
}

func (m *mockPlugin) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	m.pods[pod.Id] = pod
	err := m.runPodSandbox(m, pod)
	m.q.Add(PodSandboxEvent(pod, RunPodSandbox))
	return err
}

func (m *mockPlugin) UpdatePodSandbox(_ context.Context, pod *api.PodSandbox, overhead *api.LinuxResources, res *api.LinuxResources) error {
	m.pods[pod.Id] = pod
	m.q.Add(PodSandboxEvent(pod, UpdatePodSandbox))

	return m.updatePodSandbox(m, pod, overhead, res)
}

func (m *mockPlugin) PostUpdatePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	m.pods[pod.Id] = pod
	err := m.postUpdatePodSandbox(m, pod)
	m.q.Add(PodSandboxEvent(pod, PostUpdatePodSandbox))
	return err
}

func (m *mockPlugin) StopPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	m.pods[pod.Id] = pod
	err := m.stopPodSandbox(m, pod)
	m.q.Add(PodSandboxEvent(pod, StopPodSandbox))
	return err
}

func (m *mockPlugin) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	delete(m.pods, pod.Id)
	err := m.removePodSandbox(m, pod)
	m.q.Add(PodSandboxEvent(pod, RemovePodSandbox))
	return err
}

func (m *mockPlugin) CreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, CreateContainer))

	return m.createContainer(m, pod, ctr)
}

func (m *mockPlugin) PostCreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, PostCreateContainer))

	return m.postCreateContainer(m, pod, ctr)
}

func (m *mockPlugin) StartContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, StartContainer))

	return m.startContainer(m, pod, ctr)
}

func (m *mockPlugin) PostStartContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, PostStartContainer))

	return m.postStartContainer(m, pod, ctr)
}

func (m *mockPlugin) UpdateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container, res *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, UpdateContainer))

	return m.updateContainer(m, pod, ctr, res)
}

func (m *mockPlugin) PostUpdateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, PostUpdateContainer))

	return m.postUpdateContainer(m, pod, ctr)
}

func (m *mockPlugin) StopContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	m.pods[pod.Id] = pod
	m.ctrs[ctr.Id] = ctr
	m.q.Add(ContainerEvent(ctr, StopContainer))

	return m.stopContainer(m, pod, ctr)
}

func (m *mockPlugin) RemoveContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	delete(m.ctrs, ctr.Id)
	m.q.Add(ContainerEvent(ctr, RemoveContainer))

	return m.removeContainer(m, pod, ctr)
}

func (m *mockPlugin) ValidateContainerAdjustment(_ context.Context, req *api.ValidateContainerAdjustmentRequest) error {
	if m.validateAdjustment != nil {
		m.q.Add(ContainerEvent(req.Container, ValidateContainerAdjustment))
		return m.validateAdjustment(m, req)
	}
	return nil
}

func nopPodEvent(*mockPlugin, *api.PodSandbox) error {
	return nil
}

func nopContainerEvent(*mockPlugin, *api.PodSandbox, *api.Container) error {
	return nil
}

func nopUpdatePodSandbox(*mockPlugin, *api.PodSandbox, *api.LinuxResources, *api.LinuxResources) error {
	return nil
}

func nopCreateContainer(*mockPlugin, *api.PodSandbox, *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	return nil, nil, nil
}

func nopUpdateContainer(*mockPlugin, *api.PodSandbox, *api.Container, *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func nopStopContainer(*mockPlugin, *api.PodSandbox, *api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

type EventType string

const (
	CreateError  = "create-error"
	Started      = "started"
	Configured   = "configured"
	Synchronized = "synchronized"
	StartupError = "startup-error"
	Shutdown     = "shutdown"
	Disconnected = "closed"
	Stopped      = "stopped"

	RunPodSandbox        = "RunPodSandbox"
	UpdatePodSandbox     = "UpdatePodSandbox"
	StopPodSandbox       = "StopPodSandbox"
	RemovePodSandbox     = "RemovePodSandbox"
	PostUpdatePodSandbox = "PostUpdatePodSandbox"
	CreateContainer      = "CreateContainer"
	StartContainer       = "StartContainer"
	UpdateContainer      = "UpdateContainer"
	StopContainer        = "StopContainer"
	RemoveContainer      = "RemoveContainer"
	PostCreateContainer  = "PostCreateContainer"
	PostStartContainer   = "PostStartContainer"
	PostUpdateContainer  = "PostUpdateContainer"

	ValidateContainerAdjustment = "ValidateContainerAdjustment"

	Error   = "Error"
	Timeout = ""
)

type Event struct {
	Type EventType
	Pod  *api.PodSandbox
	Ctr  *api.Container
}

var (
	PluginCreationError = &Event{Type: CreateError}
	PluginConfigured    = &Event{Type: Configured}
	PluginSynchronized  = &Event{Type: Synchronized}
	PluginStartupError  = &Event{Type: StartupError}
	PluginShutdown      = &Event{Type: Shutdown}
	PluginDisconnected  = &Event{Type: Disconnected}
	PluginStopped       = &Event{Type: Stopped}

	PodSandboxEvent = func(pod *api.PodSandbox, t EventType) *Event {
		return &Event{Type: t, Pod: pod}
	}
	ContainerEvent = func(ctr *api.Container, t EventType) *Event {
		return &Event{Type: t, Ctr: ctr}
	}
)

func (e *Event) Matches(o *Event) bool {
	if e.Type != o.Type {
		return false
	}
	if e.Pod != nil && o.Pod != nil {
		if e.Pod.Id != o.Pod.Id {
			return false
		}
	}
	if e.Ctr != nil && o.Ctr != nil {
		if e.Ctr.Id != o.Ctr.Id || e.Ctr.PodSandboxId != o.Ctr.PodSandboxId {
			return false
		}
	}
	return true
}

func (e *Event) String() string {
	str := ""
	switch {
	case e.Ctr != nil:
		str += e.Ctr.PodSandboxId + ":" + e.Ctr.Id + "/"
	case e.Pod != nil:
		str += e.Pod.Id + "/"
	}
	return str + string(e.Type)
}

type EventQ struct {
	sync.Mutex
	q []*Event
	c chan *Event
}

func (q *EventQ) Add(e *Event) {
	if q == nil {
		return
	}
	q.Lock()
	defer q.Unlock()
	q.q = append(q.q, e)
	if q.c != nil {
		q.c <- e
	}
}

func (q *EventQ) Reset() {
	q.Lock()
	defer q.Unlock()
	q.q = []*Event{}
}

func (q *EventQ) Events() []*Event {
	q.Lock()
	defer q.Unlock()
	var events []*Event
	events = append(events, q.q...)
	return events
}

func (q *EventQ) Has(e *Event) bool {
	q.Lock()
	defer q.Unlock()
	return q.search(e) != nil
}

func (q *EventQ) search(e *Event) *Event {
	for _, qe := range q.q {
		if qe.Matches(e) {
			return qe
		}
	}
	return nil
}

func (q *EventQ) Wait(w *Event, deadline <-chan time.Time) (*Event, error) {
	var unlocked bool
	q.Lock()
	defer func() {
		if !unlocked {
			q.Unlock()
		}
	}()

	if e := q.search(w); e != nil {
		return e, nil
	}

	if q.c != nil {
		return nil, errors.New("event queue already busy Wait()ing")
	}
	q.c = make(chan *Event, 16)
	defer func() {
		c := q.c
		q.c = nil
		close(c)
	}()

	q.Unlock()
	unlocked = true
	for {
		select {
		case e := <-q.c:
			if e.Matches(w) {
				return e, nil
			}
		case <-deadline:
			return nil, fmt.Errorf("event queue timed out Wait()ing for %s", w)
		}
	}
}
