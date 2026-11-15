// Package dockertools ports Agno's DockerTools: container, image, volume and
// network management through the Docker Engine API.
//
// It talks to the Engine API directly over the Docker unix socket (or a TCP
// host via DOCKER_HOST=tcp://...), so neither the docker CLI nor the Docker Go
// SDK is required. The older agno/tools.DockerContainerManager shells out to the
// docker CLI and covers only a subset of these operations.
package dockertools

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultSocketPaths are the socket locations probed when DOCKER_HOST is unset.
func DefaultSocketPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/var/run/docker.sock",
		filepath.Join(home, ".docker", "run", "docker.sock"),
		filepath.Join(home, ".docker", "desktop", "docker.sock"),
		filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "docker.sock"),
	}
}

// DockerTools manages Docker resources via the Engine API.
type DockerTools struct {
	toolkit.Toolkit
	baseURL    string
	socketPath string
	httpClient *http.Client
}

// Option configures DockerTools.
type Option func(*DockerTools)

// WithDockerSocket uses the given unix socket path.
func WithDockerSocket(path string) Option { return func(d *DockerTools) { d.socketPath = path } }

// WithDockerBaseURL talks to an HTTP endpoint instead of a unix socket
// (e.g. "http://localhost:2375" or a test server).
func WithDockerBaseURL(u string) Option { return func(d *DockerTools) { d.baseURL = u } }

// WithDockerHTTPClient replaces the HTTP client (used with WithDockerBaseURL).
func WithDockerHTTPClient(c *http.Client) Option { return func(d *DockerTools) { d.httpClient = c } }

// Parameter structs.
type (
	ListContainersParams struct {
		All bool `json:"all,omitempty" description:"If true, show all containers (default shows just running)."`
	}
	ContainerIDParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container." required:"true"`
	}
	StopContainerParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container to stop." required:"true"`
		Timeout     int    `json:"timeout,omitempty" description:"Timeout in seconds to wait for container to stop. Default 10."`
	}
	RemoveContainerParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container to remove." required:"true"`
		Force       bool   `json:"force,omitempty" description:"If true, force the removal of a running container."`
		Volumes     bool   `json:"volumes,omitempty" description:"If true, remove anonymous volumes associated with the container."`
	}
	ContainerLogsParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container." required:"true"`
		Tail        int    `json:"tail,omitempty" description:"Number of lines to show from the end of the logs. Default 100."`
	}
	RunContainerParams struct {
		Image       string                       `json:"image" description:"The image to run." required:"true"`
		Command     string                       `json:"command,omitempty" description:"The command to run in the container."`
		Name        string                       `json:"name,omitempty" description:"A name for the container."`
		Ports       map[string]interface{}       `json:"ports,omitempty" description:"Port mappings {'container_port/protocol': host_port}."`
		Volumes     map[string]map[string]string `json:"volumes,omitempty" description:"Volume mappings {host_path_or_volume: {'bind': container_path, 'mode': 'rw'}}."`
		Environment map[string]string            `json:"environment,omitempty" description:"Environment variables."`
		Network     string                       `json:"network,omitempty" description:"Network to connect the container to."`
	}
	ExecParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container." required:"true"`
		Command     string `json:"command" description:"The command to execute." required:"true"`
	}
	NoParams        struct{}
	PullImageParams struct {
		ImageName string `json:"image_name" description:"The name of the image to pull." required:"true"`
		Tag       string `json:"tag,omitempty" description:"The tag to pull. Default latest."`
	}
	RemoveImageParams struct {
		ImageID string `json:"image_id" description:"The ID or name of the image to remove." required:"true"`
		Force   bool   `json:"force,omitempty" description:"If true, force removal of the image."`
	}
	BuildImageParams struct {
		Path       string `json:"path" description:"Path to the directory containing the Dockerfile." required:"true"`
		Tag        string `json:"tag" description:"Tag to apply to the built image." required:"true"`
		Dockerfile string `json:"dockerfile,omitempty" description:"Name of the Dockerfile. Default Dockerfile."`
	}
	TagImageParams struct {
		ImageID    string `json:"image_id" description:"The ID or name of the image to tag." required:"true"`
		Repository string `json:"repository" description:"The repository to tag in." required:"true"`
		Tag        string `json:"tag,omitempty" description:"The tag name."`
	}
	ImageIDParams struct {
		ImageID string `json:"image_id" description:"The ID or name of the image." required:"true"`
	}
	CreateVolumeParams struct {
		VolumeName string            `json:"volume_name" description:"The name of the volume to create." required:"true"`
		Driver     string            `json:"driver,omitempty" description:"The volume driver to use. Default local."`
		Labels     map[string]string `json:"labels,omitempty" description:"Labels to apply to the volume."`
	}
	RemoveVolumeParams struct {
		VolumeName string `json:"volume_name" description:"The name of the volume to remove." required:"true"`
		Force      bool   `json:"force,omitempty" description:"Force removal of the volume."`
	}
	VolumeNameParams struct {
		VolumeName string `json:"volume_name" description:"The name of the volume." required:"true"`
	}
	CreateNetworkParams struct {
		NetworkName string            `json:"network_name" description:"The name of the network to create." required:"true"`
		Driver      string            `json:"driver,omitempty" description:"The network driver to use. Default bridge."`
		Internal    bool              `json:"internal,omitempty" description:"If true, create an internal network."`
		Labels      map[string]string `json:"labels,omitempty" description:"Labels to apply to the network."`
	}
	NetworkNameParams struct {
		NetworkName string `json:"network_name" description:"The name of the network." required:"true"`
	}
	NetworkConnectParams struct {
		ContainerID string `json:"container_id" description:"The ID or name of the container." required:"true"`
		NetworkName string `json:"network_name" description:"The name of the network." required:"true"`
	}
)

// NewDockerTools creates the Docker toolkit.
func NewDockerTools(opts ...Option) *DockerTools {
	d := &DockerTools{}
	d.Toolkit = toolkit.NewToolkit()
	d.Toolkit.Name = "docker_tools"
	d.Toolkit.Description = "Manage Docker containers, images, volumes and networks through the Docker Engine API."
	for _, o := range opts {
		o(d)
	}
	d.resolveTransport()

	c := toolkit.WithConfirmation()
	r := d.Toolkit.Register
	ro := d.Toolkit.RegisterWithOptions
	r("ListContainers", "List Docker containers.", d, d.ListContainers, ListContainersParams{})
	r("StartContainer", "Start a Docker container.", d, d.StartContainer, ContainerIDParams{})
	r("StopContainer", "Stop a Docker container.", d, d.StopContainer, StopContainerParams{})
	ro("RemoveContainer", "Remove a Docker container.", d, d.RemoveContainer, RemoveContainerParams{}, c)
	r("GetContainerLogs", "Get logs from a Docker container.", d, d.GetContainerLogs, ContainerLogsParams{})
	r("InspectContainer", "Inspect a Docker container.", d, d.InspectContainer, ContainerIDParams{})
	r("RunContainer", "Run a Docker container.", d, d.RunContainer, RunContainerParams{})
	r("ExecInContainer", "Execute a command in a running container.", d, d.ExecInContainer, ExecParams{})
	r("ListImages", "List Docker images.", d, d.ListImages, NoParams{})
	r("PullImage", "Pull a Docker image.", d, d.PullImage, PullImageParams{})
	ro("RemoveImage", "Remove a Docker image.", d, d.RemoveImage, RemoveImageParams{}, c)
	r("BuildImage", "Build a Docker image from a Dockerfile.", d, d.BuildImage, BuildImageParams{})
	r("TagImage", "Tag a Docker image.", d, d.TagImage, TagImageParams{})
	r("InspectImage", "Inspect a Docker image.", d, d.InspectImage, ImageIDParams{})
	r("ListVolumes", "List Docker volumes.", d, d.ListVolumes, NoParams{})
	r("CreateVolume", "Create a Docker volume.", d, d.CreateVolume, CreateVolumeParams{})
	ro("RemoveVolume", "Remove a Docker volume.", d, d.RemoveVolume, RemoveVolumeParams{}, c)
	r("InspectVolume", "Inspect a Docker volume.", d, d.InspectVolume, VolumeNameParams{})
	r("ListNetworks", "List Docker networks.", d, d.ListNetworks, NoParams{})
	r("CreateNetwork", "Create a Docker network.", d, d.CreateNetwork, CreateNetworkParams{})
	ro("RemoveNetwork", "Remove a Docker network.", d, d.RemoveNetwork, NetworkNameParams{}, c)
	r("InspectNetwork", "Inspect a Docker network.", d, d.InspectNetwork, NetworkNameParams{})
	r("ConnectContainerToNetwork", "Connect a container to a network.", d, d.ConnectContainerToNetwork, NetworkConnectParams{})
	r("DisconnectContainerFromNetwork", "Disconnect a container from a network.", d, d.DisconnectContainerFromNetwork, NetworkConnectParams{})
	return d
}

func (d *DockerTools) resolveTransport() {
	if d.baseURL == "" && d.socketPath == "" {
		if h := os.Getenv("DOCKER_HOST"); h != "" {
			switch {
			case strings.HasPrefix(h, "unix://"):
				d.socketPath = strings.TrimPrefix(h, "unix://")
			case strings.HasPrefix(h, "tcp://"):
				d.baseURL = "http://" + strings.TrimPrefix(h, "tcp://")
			default:
				d.baseURL = h
			}
		}
	}
	if d.baseURL == "" && d.socketPath == "" {
		for _, p := range DefaultSocketPaths() {
			if _, err := os.Stat(p); err == nil {
				d.socketPath = p
				break
			}
		}
		if d.socketPath == "" {
			d.socketPath = "/var/run/docker.sock"
		}
	}
	if d.baseURL == "" {
		d.baseURL = "http://docker"
		if d.httpClient == nil {
			sock := d.socketPath
			d.httpClient = &http.Client{Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var dialer net.Dialer
					return dialer.DialContext(ctx, "unix", sock)
				},
			}}
		}
	}
	if d.httpClient == nil {
		d.httpClient = &http.Client{}
	}
}

func (d *DockerTools) client() *restapi.Client { return restapi.New(d.baseURL, d.httpClient, nil) }

func (d *DockerTools) do(method, path string, q url.Values, body interface{}) ([]byte, error) {
	return d.client().Do(context.Background(), method, path, q, body)
}

func errMsg(prefix string, err error) string {
	var apiErr *restapi.APIError
	if errors.As(err, &apiErr) {
		var m struct {
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(apiErr.Body), &m) == nil && m.Message != "" {
			return fmt.Sprintf("%s: %d %s", prefix, apiErr.Status, m.Message)
		}
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}

func esc(s string) string { return url.PathEscape(s) }

func boolQ(b bool) string { return strconv.FormatBool(b) }

// ListContainers lists containers.
func (d *DockerTools) ListContainers(p ListContainersParams) (interface{}, error) {
	data, err := d.do("GET", "/containers/json", url.Values{"all": {boolQ(p.All)}}, nil)
	if err != nil {
		return errMsg("Error listing containers", err), nil
	}
	var raw []map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	out := make([]map[string]interface{}, 0, len(raw))
	for _, c := range raw {
		name := ""
		if names, ok := c["Names"].([]interface{}); ok && len(names) > 0 {
			name = strings.TrimPrefix(fmt.Sprint(names[0]), "/")
		}
		out = append(out, map[string]interface{}{
			"id": c["Id"], "name": name, "image": c["Image"], "status": c["State"],
			"created": c["Created"], "ports": c["Ports"], "labels": c["Labels"],
		})
	}
	return restapi.ToJSON(out), nil
}

// StartContainer starts a container.
func (d *DockerTools) StartContainer(p ContainerIDParams) (interface{}, error) {
	if _, err := d.do("POST", "/containers/"+esc(p.ContainerID)+"/start", nil, nil); err != nil {
		return errMsg("Error starting container", err), nil
	}
	return fmt.Sprintf("Container %s started successfully", p.ContainerID), nil
}

// StopContainer stops a container.
func (d *DockerTools) StopContainer(p StopContainerParams) (interface{}, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	if _, err := d.do("POST", "/containers/"+esc(p.ContainerID)+"/stop", url.Values{"t": {strconv.Itoa(timeout)}}, nil); err != nil {
		return errMsg("Error stopping container", err), nil
	}
	return fmt.Sprintf("Container %s stopped successfully", p.ContainerID), nil
}

// RemoveContainer removes a container.
func (d *DockerTools) RemoveContainer(p RemoveContainerParams) (interface{}, error) {
	q := url.Values{"force": {boolQ(p.Force)}, "v": {boolQ(p.Volumes)}}
	if _, err := d.do("DELETE", "/containers/"+esc(p.ContainerID), q, nil); err != nil {
		return errMsg("Error removing container", err), nil
	}
	return fmt.Sprintf("Container %s removed successfully", p.ContainerID), nil
}

// GetContainerLogs returns the last lines of a container's logs.
func (d *DockerTools) GetContainerLogs(p ContainerLogsParams) (interface{}, error) {
	tail := p.Tail
	if tail <= 0 {
		tail = 100
	}
	q := url.Values{"stdout": {"true"}, "stderr": {"true"}, "tail": {strconv.Itoa(tail)}}
	data, err := d.do("GET", "/containers/"+esc(p.ContainerID)+"/logs", q, nil)
	if err != nil {
		return errMsg("Error getting container logs", err), nil
	}
	logs := demux(data)
	if logs == "" {
		return "No logs found", nil
	}
	return logs, nil
}

// demux strips Docker's 8-byte stream multiplexing headers when present.
func demux(data []byte) string {
	var out bytes.Buffer
	rest := data
	for len(rest) >= 8 {
		typ := rest[0]
		if typ > 2 || rest[1] != 0 || rest[2] != 0 || rest[3] != 0 {
			return string(data) // not multiplexed (TTY)
		}
		size := int(binary.BigEndian.Uint32(rest[4:8]))
		if 8+size > len(rest) {
			return string(data)
		}
		out.Write(rest[8 : 8+size])
		rest = rest[8+size:]
	}
	if len(rest) != 0 {
		return string(data)
	}
	return out.String()
}

func (d *DockerTools) inspect(path, prefix string) (interface{}, error) {
	data, err := d.do("GET", path, nil, nil)
	if err != nil {
		return errMsg(prefix, err), nil
	}
	return restapi.Pretty(data), nil
}

// InspectContainer returns the container attributes.
func (d *DockerTools) InspectContainer(p ContainerIDParams) (interface{}, error) {
	return d.inspect("/containers/"+esc(p.ContainerID)+"/json", "Error inspecting container")
}

// RunContainer creates and starts a container. Containers always run
// detached (Python's default), and the container ID is returned.
func (d *DockerTools) RunContainer(p RunContainerParams) (interface{}, error) {
	cfg := map[string]interface{}{"Image": p.Image}
	if p.Command != "" {
		cfg["Cmd"] = splitCommand(p.Command)
	}
	if len(p.Environment) > 0 {
		env := make([]string, 0, len(p.Environment))
		for k, v := range p.Environment {
			env = append(env, k+"="+v)
		}
		cfg["Env"] = env
	}
	host := map[string]interface{}{}
	if len(p.Ports) > 0 {
		exposed := map[string]struct{}{}
		bindings := map[string][]map[string]string{}
		for cport, hport := range p.Ports {
			if !strings.Contains(cport, "/") {
				cport += "/tcp"
			}
			exposed[cport] = struct{}{}
			hp := fmt.Sprint(hport)
			if f, ok := hport.(float64); ok {
				hp = strconv.Itoa(int(f))
			}
			bindings[cport] = []map[string]string{{"HostPort": hp}}
		}
		cfg["ExposedPorts"] = exposed
		host["PortBindings"] = bindings
	}
	if len(p.Volumes) > 0 {
		var binds []string
		for src, spec := range p.Volumes {
			b := src + ":" + spec["bind"]
			if m := spec["mode"]; m != "" {
				b += ":" + m
			}
			binds = append(binds, b)
		}
		host["Binds"] = binds
	}
	if p.Network != "" {
		host["NetworkMode"] = p.Network
	}
	cfg["HostConfig"] = host
	var q url.Values
	if p.Name != "" {
		q = url.Values{"name": {p.Name}}
	}
	data, err := d.do("POST", "/containers/create", q, cfg)
	var apiErr *restapi.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		// Image missing locally: pull it then retry, like docker-py's run.
		img, tag := splitImage(p.Image)
		if _, perr := d.do("POST", "/images/create", url.Values{"fromImage": {img}, "tag": {tag}}, nil); perr == nil {
			data, err = d.do("POST", "/containers/create", q, cfg)
		}
	}
	if err != nil {
		return errMsg("Error running container", err), nil
	}
	var created struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(data, &created)
	if _, err := d.do("POST", "/containers/"+created.ID+"/start", nil, nil); err != nil {
		return errMsg("Error running container", err), nil
	}
	return fmt.Sprintf("Container started with ID: %s", created.ID), nil
}

// ExecInContainer runs a command inside a running container.
func (d *DockerTools) ExecInContainer(p ExecParams) (interface{}, error) {
	data, err := d.do("POST", "/containers/"+esc(p.ContainerID)+"/exec", nil, map[string]interface{}{
		"Cmd": splitCommand(p.Command), "AttachStdout": true, "AttachStderr": true,
	})
	if err != nil {
		return errMsg("Error executing command in container", err), nil
	}
	var ex struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(data, &ex)
	out, err := d.do("POST", "/exec/"+ex.ID+"/start", nil, map[string]interface{}{"Detach": false, "Tty": false})
	if err != nil {
		return errMsg("Error executing command in container", err), nil
	}
	output := demux(out)
	info, err := d.do("GET", "/exec/"+ex.ID+"/json", nil, nil)
	if err != nil {
		return errMsg("Error executing command in container", err), nil
	}
	var st struct {
		ExitCode int `json:"ExitCode"`
	}
	_ = json.Unmarshal(info, &st)
	if st.ExitCode == 0 {
		return output, nil
	}
	return fmt.Sprintf("Command failed with exit code %d: %s", st.ExitCode, output), nil
}

// ListImages lists images.
func (d *DockerTools) ListImages(_ NoParams) (interface{}, error) {
	data, err := d.do("GET", "/images/json", nil, nil)
	if err != nil {
		return errMsg("Error listing images", err), nil
	}
	var raw []map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	out := make([]map[string]interface{}, 0, len(raw))
	for _, i := range raw {
		tags := i["RepoTags"]
		if tags == nil {
			tags = []interface{}{}
		}
		out = append(out, map[string]interface{}{
			"id": i["Id"], "tags": tags, "created": i["Created"], "size": i["Size"], "labels": i["Labels"],
		})
	}
	return restapi.ToJSON(out), nil
}

// PullImage pulls an image.
func (d *DockerTools) PullImage(p PullImageParams) (interface{}, error) {
	tag := p.Tag
	if tag == "" {
		tag = "latest"
	}
	data, err := d.do("POST", "/images/create", url.Values{"fromImage": {p.ImageName}, "tag": {tag}}, nil)
	if err != nil {
		return errMsg("Error pulling image", err), nil
	}
	if msg := streamError(data); msg != "" {
		return "Error pulling image: " + msg, nil
	}
	return fmt.Sprintf("Image %s:%s pulled successfully", p.ImageName, tag), nil
}

// streamError finds an "error" entry in a JSON-lines progress stream.
func streamError(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var line map[string]interface{}
		if json.Unmarshal(sc.Bytes(), &line) == nil {
			if e, ok := line["error"].(string); ok && e != "" {
				return e
			}
		}
	}
	return ""
}

// RemoveImage removes an image.
func (d *DockerTools) RemoveImage(p RemoveImageParams) (interface{}, error) {
	_, err := d.do("DELETE", "/images/"+esc(p.ImageID), url.Values{"force": {boolQ(p.Force)}}, nil)
	var apiErr *restapi.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return fmt.Sprintf("Image %s not found", p.ImageID), nil
	}
	if err != nil {
		return errMsg("Error removing image", err), nil
	}
	return fmt.Sprintf("Image %s removed successfully", p.ImageID), nil
}

// BuildImage builds an image from a local directory.
func (d *DockerTools) BuildImage(p BuildImageParams) (interface{}, error) {
	dockerfile := p.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	tarball, err := tarDir(p.Path)
	if err != nil {
		return errMsg("Error building image", err), nil
	}
	q := url.Values{"t": {p.Tag}, "dockerfile": {dockerfile}, "rm": {"true"}}
	req, err := http.NewRequest("POST", d.client().URL("/build", q), bytes.NewReader(tarball))
	if err != nil {
		return errMsg("Error building image", err), nil
	}
	req.Header.Set("Content-Type", "application/x-tar")
	data, err := d.client().Send(req)
	if err != nil {
		return errMsg("Error building image", err), nil
	}
	if msg := streamError(data); msg != "" {
		return "Error building image: " + msg, nil
	}
	id := p.Tag
	if info, err := d.do("GET", "/images/"+esc(p.Tag)+"/json", nil, nil); err == nil {
		var img struct {
			ID string `json:"Id"`
		}
		if json.Unmarshal(info, &img) == nil && img.ID != "" {
			id = img.ID
		}
	}
	return fmt.Sprintf("Image built successfully with ID: %s", id), nil
}

func tarDir(root string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TagImage tags an image.
func (d *DockerTools) TagImage(p TagImageParams) (interface{}, error) {
	q := url.Values{"repo": {p.Repository}}
	if p.Tag != "" {
		q.Set("tag", p.Tag)
	}
	if _, err := d.do("POST", "/images/"+esc(p.ImageID)+"/tag", q, nil); err != nil {
		return errMsg("Error tagging image", err), nil
	}
	tag := p.Tag
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("Image %s tagged as %s:%s", p.ImageID, p.Repository, tag), nil
}

// InspectImage returns the image attributes.
func (d *DockerTools) InspectImage(p ImageIDParams) (interface{}, error) {
	return d.inspect("/images/"+esc(p.ImageID)+"/json", "Error inspecting image")
}

// ListVolumes lists volumes.
func (d *DockerTools) ListVolumes(_ NoParams) (interface{}, error) {
	data, err := d.do("GET", "/volumes", nil, nil)
	if err != nil {
		return errMsg("Error listing volumes", err), nil
	}
	var raw struct {
		Volumes []map[string]interface{} `json:"Volumes"`
	}
	_ = json.Unmarshal(data, &raw)
	out := make([]map[string]interface{}, 0, len(raw.Volumes))
	for _, v := range raw.Volumes {
		labels := v["Labels"]
		if labels == nil {
			labels = map[string]interface{}{}
		}
		out = append(out, map[string]interface{}{
			"name": v["Name"], "driver": v["Driver"], "mountpoint": v["Mountpoint"], "created": v["CreatedAt"], "labels": labels,
		})
	}
	return restapi.ToJSON(out), nil
}

// CreateVolume creates a volume.
func (d *DockerTools) CreateVolume(p CreateVolumeParams) (interface{}, error) {
	driver := p.Driver
	if driver == "" {
		driver = "local"
	}
	body := map[string]interface{}{"Name": p.VolumeName, "Driver": driver}
	if p.Labels != nil {
		body["Labels"] = p.Labels
	}
	if _, err := d.do("POST", "/volumes/create", nil, body); err != nil {
		return errMsg("Error creating volume", err), nil
	}
	return fmt.Sprintf("Volume %s created successfully", p.VolumeName), nil
}

// RemoveVolume removes a volume.
func (d *DockerTools) RemoveVolume(p RemoveVolumeParams) (interface{}, error) {
	if _, err := d.do("DELETE", "/volumes/"+esc(p.VolumeName), url.Values{"force": {boolQ(p.Force)}}, nil); err != nil {
		return errMsg("Error removing volume", err), nil
	}
	return fmt.Sprintf("Volume %s removed successfully", p.VolumeName), nil
}

// InspectVolume returns the volume attributes.
func (d *DockerTools) InspectVolume(p VolumeNameParams) (interface{}, error) {
	return d.inspect("/volumes/"+esc(p.VolumeName), "Error inspecting volume")
}

// ListNetworks lists networks.
func (d *DockerTools) ListNetworks(_ NoParams) (interface{}, error) {
	data, err := d.do("GET", "/networks", nil, nil)
	if err != nil {
		return errMsg("Error listing networks", err), nil
	}
	var raw []map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	out := make([]map[string]interface{}, 0, len(raw))
	for _, n := range raw {
		containers := []string{}
		if cs, ok := n["Containers"].(map[string]interface{}); ok {
			for id := range cs {
				containers = append(containers, id)
			}
		}
		internal, _ := n["Internal"].(bool)
		out = append(out, map[string]interface{}{
			"id": n["Id"], "name": n["Name"], "driver": n["Driver"], "scope": n["Scope"],
			"created": n["Created"], "internal": internal, "containers": containers,
		})
	}
	return restapi.ToJSON(out), nil
}

// CreateNetwork creates a network.
func (d *DockerTools) CreateNetwork(p CreateNetworkParams) (interface{}, error) {
	driver := p.Driver
	if driver == "" {
		driver = "bridge"
	}
	body := map[string]interface{}{"Name": p.NetworkName, "Driver": driver, "Internal": p.Internal}
	if p.Labels != nil {
		body["Labels"] = p.Labels
	}
	data, err := d.do("POST", "/networks/create", nil, body)
	if err != nil {
		return errMsg("Error creating network", err), nil
	}
	var created struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(data, &created)
	return fmt.Sprintf("Network %s created successfully with ID: %s", p.NetworkName, created.ID), nil
}

// RemoveNetwork removes a network.
func (d *DockerTools) RemoveNetwork(p NetworkNameParams) (interface{}, error) {
	if _, err := d.do("DELETE", "/networks/"+esc(p.NetworkName), nil, nil); err != nil {
		return errMsg("Error removing network", err), nil
	}
	return fmt.Sprintf("Network %s removed successfully", p.NetworkName), nil
}

// InspectNetwork returns the network attributes.
func (d *DockerTools) InspectNetwork(p NetworkNameParams) (interface{}, error) {
	return d.inspect("/networks/"+esc(p.NetworkName), "Error inspecting network")
}

// ConnectContainerToNetwork connects a container to a network.
func (d *DockerTools) ConnectContainerToNetwork(p NetworkConnectParams) (interface{}, error) {
	if _, err := d.do("POST", "/networks/"+esc(p.NetworkName)+"/connect", nil, map[string]string{"Container": p.ContainerID}); err != nil {
		return errMsg("Error connecting container to network", err), nil
	}
	return fmt.Sprintf("Container %s connected to network %s", p.ContainerID, p.NetworkName), nil
}

// DisconnectContainerFromNetwork disconnects a container from a network.
func (d *DockerTools) DisconnectContainerFromNetwork(p NetworkConnectParams) (interface{}, error) {
	if _, err := d.do("POST", "/networks/"+esc(p.NetworkName)+"/disconnect", nil, map[string]string{"Container": p.ContainerID}); err != nil {
		return errMsg("Error disconnecting container from network", err), nil
	}
	return fmt.Sprintf("Container %s disconnected from network %s", p.ContainerID, p.NetworkName), nil
}

// splitCommand splits a command line like a POSIX shell (quotes and escapes).
func splitCommand(s string) []string {
	var args []string
	var cur strings.Builder
	inArg := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, inArg = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inArg = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args
}

func splitImage(image string) (string, string) {
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		return image[:i], image[i+1:]
	}
	return image, "latest"
}
