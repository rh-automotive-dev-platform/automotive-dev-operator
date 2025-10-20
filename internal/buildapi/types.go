package buildapi

import (
	"fmt"
	"strings"
)

type Distro string

func (d Distro) IsValid() bool {
	return strings.TrimSpace(string(d)) != ""
}

type Target string

func (t Target) IsValid() bool {
	return strings.TrimSpace(string(t)) != ""
}

type Architecture string

func (a Architecture) IsValid() bool {
	return strings.TrimSpace(string(a)) != ""
}

type ExportFormat string

func (e ExportFormat) IsValid() bool {
	return strings.TrimSpace(string(e)) != ""
}

type Mode string

func (m Mode) IsValid() bool {
	return strings.TrimSpace(string(m)) != ""
}

func ParseDistro(s string) (Distro, error) {
	d := Distro(s)
	if !d.IsValid() {
		return "", fmt.Errorf("distro cannot be empty")
	}
	return d, nil
}

func ParseTarget(s string) (Target, error) {
	t := Target(s)
	if !t.IsValid() {
		return "", fmt.Errorf("target cannot be empty")
	}
	return t, nil
}

func ParseArchitecture(s string) (Architecture, error) {
	a := Architecture(s)
	if !a.IsValid() {
		return "", fmt.Errorf("architecture cannot be empty")
	}
	return a, nil
}

func ParseExportFormat(s string) (ExportFormat, error) {
	e := ExportFormat(s)
	if !e.IsValid() {
		return "", fmt.Errorf("exportFormat cannot be empty")
	}
	return e, nil
}

func ParseMode(s string) (Mode, error) {
	m := Mode(s)
	if !m.IsValid() {
		return "", fmt.Errorf("mode cannot be empty")
	}
	return m, nil
}

// BuildRequest is the payload to create a build via the REST API
type BuildRequest struct {
	Name                   string               `json:"name"`
	Manifest               string               `json:"manifest"`
	ManifestFileName       string               `json:"manifestFileName"`
	Distro                 Distro               `json:"distro"`
	Target                 Target               `json:"target"`
	Architecture           Architecture         `json:"architecture"`
	ExportFormat           ExportFormat         `json:"exportFormat"`
	Mode                   Mode                 `json:"mode"`
	AutomotiveImageBuilder string               `json:"automotiveImageBuilder"`
	StorageClass           string               `json:"storageClass"`
	CustomDefs             []string             `json:"customDefs"`
	AIBExtraArgs           []string             `json:"aibExtraArgs"`
	AIBOverrideArgs        []string             `json:"aibOverrideArgs"`
	ServeArtifact          bool                 `json:"serveArtifact"`
	Compression            string               `json:"compression,omitempty"`
	RegistryCredentials    *RegistryCredentials `json:"registryCredentials,omitempty"`
}

type RegistryCredentials struct {
	Enabled      bool   `json:"enabled"`
	AuthType     string `json:"authType"`
	RegistryURL  string `json:"registryUrl"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Token        string `json:"token"`
	DockerConfig string `json:"dockerConfig"`
}

// BuildResponse is returned by POST and GET build operations
type BuildResponse struct {
	Name             string `json:"name"`
	Phase            string `json:"phase"`
	Message          string `json:"message"`
	RequestedBy      string `json:"requestedBy,omitempty"`
	ArtifactURL      string `json:"artifactURL,omitempty"`
	ArtifactFileName string `json:"artifactFileName,omitempty"`
	StartTime        string `json:"startTime,omitempty"`
	CompletionTime   string `json:"completionTime,omitempty"`
}

// BuildListItem represents a build in the list API
type BuildListItem struct {
	Name           string `json:"name"`
	Phase          string `json:"phase"`
	Message        string `json:"message"`
	RequestedBy    string `json:"requestedBy,omitempty"`
	CreatedAt      string `json:"createdAt"`
	StartTime      string `json:"startTime,omitempty"`
	CompletionTime string `json:"completionTime,omitempty"`
}

type (
	BuildRequestAlias  = BuildRequest
	BuildListItemAlias = BuildListItem
)

// BuildTemplateResponse includes the original inputs plus a hint of source files referenced by the manifest
type BuildTemplateResponse struct {
	BuildRequest `json:",inline"`
	SourceFiles  []string `json:"sourceFiles,omitempty"`
}
