package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	buildapitypes "github.com/rh-sdv-cloud-incubator/automotive-dev-operator/internal/buildapi"
	buildapiclient "github.com/rh-sdv-cloud-incubator/automotive-dev-operator/internal/buildapi/client"
	progressbar "github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	serverURL              string
	imageBuildCfg          string
	manifest               string
	buildName              string
	distro                 string
	target                 string
	architecture           string
	exportFormat           string
	mode                   string
	automotiveImageBuilder string
	storageClass           string
	outputDir              string
	timeout                int
	waitForBuild           bool
	download               bool
	customDefs             []string
	followLogs             bool
	version                string
	aibExtraArgs           string
	aibOverrideArgs        string
	compressArtifacts      bool
	compressionAlgo        string
	authToken              string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "caib",
		Short:   "Cloud Automotive Image Builder",
		Version: version,
	}

	rootCmd.InitDefaultVersionFlag()
	rootCmd.SetVersionTemplate("caib version: {{.Version}}\n")

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Create an ImageBuild resource",
		Run:   runBuild,
	}

	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download artifacts from a completed build",
		Run:   runDownload,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List existing ImageBuilds",
		Run:   runList,
	}

	buildCmd.Flags().StringVar(&serverURL, "server", os.Getenv("CAIB_SERVER"), "REST API server base URL (e.g. https://api.example)")
	buildCmd.Flags().StringVar(&authToken, "token", os.Getenv("CAIB_TOKEN"), "Bearer token for authentication (e.g., OpenShift access token)")
	buildCmd.Flags().StringVar(&imageBuildCfg, "config", "", "path to ImageBuild YAML configuration file")
	buildCmd.Flags().StringVar(&manifest, "manifest", "", "path to manifest YAML file for the build")
	buildCmd.Flags().StringVar(&buildName, "name", "", "name for the ImageBuild")
	buildCmd.Flags().StringVar(&distro, "distro", "autosd", "distribution to build")
	buildCmd.Flags().StringVar(&target, "target", "qemu", "target platform (qemu, etc)")
	buildCmd.Flags().StringVar(&architecture, "arch", "arm64", "architecture (amd64, arm64)")
	buildCmd.Flags().StringVar(&exportFormat, "export-format", "image", "export format (image, qcow2, etc)")
	buildCmd.Flags().StringVar(&mode, "mode", "image", "build mode")
	buildCmd.Flags().StringVar(&automotiveImageBuilder, "automotive-image-builder", "quay.io/centos-sig-automotive/automotive-image-builder:1.0.0", "container image for automotive-image-builder")
	buildCmd.Flags().StringVar(&storageClass, "storage-class", "", "storage class to use for build workspace PVC")
	buildCmd.Flags().IntVar(&timeout, "timeout", 60, "timeout in minutes when waiting for build completion")
	buildCmd.Flags().BoolVarP(&waitForBuild, "wait", "w", false, "wait for the build to complete")
	buildCmd.Flags().BoolVarP(&download, "download", "d", false, "automatically download artifacts when build completes")
	buildCmd.Flags().BoolVar(&compressArtifacts, "compress", true, "compress directory artifacts (tar.gz). For directories, server always compresses.")
	buildCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "follow logs of the build")
	buildCmd.Flags().StringArrayVar(&customDefs, "define", []string{}, "Custom definition in KEY=VALUE format (can be specified multiple times)")
	buildCmd.Flags().StringVar(&aibExtraArgs, "aib-args", "", "extra arguments passed to automotive-image-builder (space-separated)")
	buildCmd.Flags().StringVar(&aibOverrideArgs, "override", "", "override arguments passed as-is to automotive-image-builder")
	buildCmd.Flags().StringVar(&compressionAlgo, "compression", "gzip", "artifact compression algorithm (lz4|gzip)")
	_ = buildCmd.MarkFlagRequired("arch")

	downloadCmd.Flags().StringVar(&serverURL, "server", os.Getenv("CAIB_SERVER"), "REST API server base URL (e.g. https://api.example)")
	downloadCmd.Flags().StringVar(&authToken, "token", os.Getenv("CAIB_TOKEN"), "Bearer token for authentication (e.g., OpenShift access token)")
	downloadCmd.Flags().StringVar(&buildName, "name", "", "name of the ImageBuild")
	downloadCmd.Flags().StringVar(&outputDir, "output-dir", "./output", "directory to save artifacts")
	downloadCmd.MarkFlagRequired("name")
	downloadCmd.Flags().BoolVar(&compressArtifacts, "compress", true, "compress directory artifacts (tar.gz). For directories, server always compresses.")

	listCmd.Flags().StringVar(&serverURL, "server", os.Getenv("CAIB_SERVER"), "REST API server base URL (e.g. https://api.example)")
	listCmd.Flags().StringVar(&authToken, "token", os.Getenv("CAIB_TOKEN"), "Bearer token for authentication (e.g., OpenShift access token)")

	rootCmd.AddCommand(buildCmd, downloadCmd, listCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runBuild(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	if err := validateBuildRequirements(); err != nil {
		handleError(err)
	}

	if serverURL == "" {
		handleError(fmt.Errorf("--server is required"))
	}

	if serverURL != "" {
		if strings.TrimSpace(authToken) == "" {
			if tok, err := loadTokenFromKubeconfig(); err == nil && strings.TrimSpace(tok) != "" {
				authToken = tok
			}
		}
		var opts []buildapiclient.Option
		if strings.TrimSpace(authToken) != "" {
			opts = append(opts, buildapiclient.WithAuthToken(strings.TrimSpace(authToken)))
		}
		api, err := buildapiclient.New(serverURL, opts...)
		if err != nil {
			handleError(err)
		}

		manifestBytes, err := os.ReadFile(manifest)
		if err != nil {
			handleError(fmt.Errorf("error reading manifest: %w", err))
		}

		parsedDistro, err := buildapitypes.ParseDistro(distro)
		if err != nil {
			handleError(err)
		}
		parsedTarget, err := buildapitypes.ParseTarget(target)
		if err != nil {
			handleError(err)
		}
		parsedArch, err := buildapitypes.ParseArchitecture(architecture)
		if err != nil {
			handleError(err)
		}
		parsedExportFormat, err := buildapitypes.ParseExportFormat(exportFormat)
		if err != nil {
			handleError(err)
		}
		parsedMode, err := buildapitypes.ParseMode(mode)
		if err != nil {
			handleError(err)
		}

		var aibArgsArray []string
		var aibOverrideArray []string
		if strings.TrimSpace(aibExtraArgs) != "" {
			aibArgsArray = strings.Fields(aibExtraArgs)
		}
		if strings.TrimSpace(aibOverrideArgs) != "" {
			aibOverrideArray = strings.Fields(aibOverrideArgs)
		}

		req := buildapitypes.BuildRequest{
			Name:                   buildName,
			Manifest:               string(manifestBytes),
			ManifestFileName:       filepath.Base(manifest),
			Distro:                 parsedDistro,
			Target:                 parsedTarget,
			Architecture:           parsedArch,
			ExportFormat:           parsedExportFormat,
			Mode:                   parsedMode,
			AutomotiveImageBuilder: automotiveImageBuilder,
			StorageClass:           storageClass,
			CustomDefs:             customDefs,
			AIBExtraArgs:           aibArgsArray,
			AIBOverrideArgs:        aibOverrideArray,
			ServeArtifact:          download,
			Compression:            compressionAlgo,
		}

		resp, err := api.CreateBuild(ctx, req)
		if err != nil {
			handleError(err)
		}
		fmt.Printf("Build %s accepted: %s - %s\n", resp.Name, resp.Phase, resp.Message)
		// If manifest references local files, upload them via the API
		localRefs, err := findLocalFileReferences(string(manifestBytes))
		if err != nil {
			handleError(fmt.Errorf("manifest file reference error: %w", err))
		}
		if len(localRefs) > 0 {
			for _, ref := range localRefs {
				if _, err := os.Stat(ref["source_path"]); err != nil {
					handleError(fmt.Errorf("referenced file %s does not exist: %w", ref["source_path"], err))
				}
			}

			fmt.Println("Waiting for upload server to be ready...")
			readyCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			for {
				if err := readyCtx.Err(); err != nil {
					handleError(fmt.Errorf("timed out waiting for upload server to be ready"))
				}
				reqCtx, c := context.WithTimeout(ctx, 15*time.Second)
				st, err := api.GetBuild(reqCtx, resp.Name)
				c()
				if err == nil {
					if st.Phase == "Uploading" {
						break
					}
					if st.Phase == "Failed" {
						handleError(fmt.Errorf("build failed while waiting for upload server: %s", st.Message))
					}
				}
				time.Sleep(3 * time.Second)
			}

			uploads := make([]buildapiclient.Upload, 0, len(localRefs))
			for _, ref := range localRefs {
				uploads = append(uploads, buildapiclient.Upload{SourcePath: ref["source_path"], DestPath: ref["source_path"]})
			}

			uploadDeadline := time.Now().Add(10 * time.Minute)
			for {
				if err := api.UploadFiles(ctx, resp.Name, uploads); err != nil {
					lower := strings.ToLower(err.Error())
					if time.Now().After(uploadDeadline) {
						handleError(fmt.Errorf("upload files failed: %w", err))
					}
					if strings.Contains(lower, "503") || strings.Contains(lower, "service unavailable") || strings.Contains(lower, "upload pod not ready") {
						fmt.Println("Upload server not ready yet. Retrying...")
						time.Sleep(5 * time.Second)
						continue
					}
					handleError(fmt.Errorf("upload files failed: %w", err))
				}
				break
			}
			fmt.Println("Local files uploaded. Build will proceed.")
		}

		if waitForBuild || followLogs || download {
			fmt.Println("Waiting for build to complete...")
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
			defer cancel()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			userFollowRequested := followLogs
			var lastPhase, lastMessage string
			logFollowWarned := false

			logClient := &http.Client{
				Timeout: 10 * time.Minute,
				Transport: &http.Transport{
					ResponseHeaderTimeout: 30 * time.Second,
					IdleConnTimeout:       2 * time.Minute,
				},
			}

			for {
				select {
				case <-timeoutCtx.Done():
					handleError(fmt.Errorf("timed out waiting for build"))
				case <-ticker.C:
					if followLogs {
						req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serverURL, "/")+"/v1/builds/"+url.PathEscape(resp.Name)+"/logs?follow=1", nil)
						if strings.TrimSpace(authToken) != "" {
							req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(authToken))
						}
						resp2, err := logClient.Do(req)
						if err == nil && resp2.StatusCode == http.StatusOK {
							fmt.Println("Streaming logs...")
							io.Copy(os.Stdout, resp2.Body)
							resp2.Body.Close()
							followLogs = userFollowRequested
						} else if resp2 != nil {
							body, _ := io.ReadAll(resp2.Body)
							msg := strings.TrimSpace(string(body))
							if resp2.StatusCode == http.StatusServiceUnavailable || resp2.StatusCode == http.StatusGatewayTimeout {
								if !logFollowWarned {
									fmt.Println("log stream not ready (HTTP", resp2.StatusCode, "). Retrying…")
									logFollowWarned = true
								}
								// treat as transient; keep trying silently afterwards
							} else {
								if msg != "" {
									fmt.Printf("log stream error (%d): %s\n", resp2.StatusCode, msg)
								} else {
									fmt.Printf("log stream error: HTTP %d\n", resp2.StatusCode)
								}
								followLogs = false
							}
							resp2.Body.Close()
						}
					}
					reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
					st, err := api.GetBuild(reqCtx, resp.Name)
					cancel()
					if err != nil {
						fmt.Printf("status check failed: %v\n", err)
						continue
					}
					if !userFollowRequested {
						if st.Phase != lastPhase || st.Message != lastMessage {
							fmt.Printf("status: %s - %s\n", st.Phase, st.Message)
							lastPhase = st.Phase
							lastMessage = st.Message
						}
					}
					if st.Phase == "Completed" {
						if download {
							if err := downloadArtifactViaAPI(ctx, serverURL, resp.Name, outputDir); err != nil {
								fmt.Printf("Download via API failed: %v\n", err)
							}
							return
						}
						return
					}
					if st.Phase == "Failed" {
						handleError(fmt.Errorf("build failed: %s", st.Message))
					}
				}
			}
		}
		return
	}

}

func validateBuildRequirements() error {
	if manifest == "" {
		return fmt.Errorf("--manifest is required")
	}

	if buildName == "" {
		return fmt.Errorf("name is required")
	}

	if strings.TrimSpace(architecture) == "" {
		return fmt.Errorf("--arch is required")
	}

	return nil
}

func handleError(err error) {
	fmt.Printf("Error: %v\n", err)
	os.Exit(1)
}

func findLocalFileReferences(manifestContent string) ([]map[string]string, error) {
	var manifestData map[string]any
	var localFiles []map[string]string

	if err := yaml.Unmarshal([]byte(manifestContent), &manifestData); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	isPathSafe := func(path string) error {
		if path == "" || path == "/" {
			return fmt.Errorf("empty or root path is not allowed")
		}

		if strings.Contains(path, "..") {
			return fmt.Errorf("directory traversal detected in path: %s", path)
		}

		if filepath.IsAbs(path) {
			// TODO add safe dirs flag
			safeDirectories := []string{}
			isInSafeDir := false
			for _, dir := range safeDirectories {
				if strings.HasPrefix(path, dir+"/") {
					isInSafeDir = true
					break
				}
			}
			if !isInSafeDir {
				return fmt.Errorf("absolute path outside safe directories: %s", path)
			}
		}

		return nil
	}

	processAddFiles := func(addFiles []any) error {
		for _, file := range addFiles {
			if fileMap, ok := file.(map[string]any); ok {
				path, hasPath := fileMap["path"].(string)
				sourcePath, hasSourcePath := fileMap["source_path"].(string)
				if hasPath && hasSourcePath {
					if err := isPathSafe(sourcePath); err != nil {
						return err
					}
					localFiles = append(localFiles, map[string]string{
						"path":        path,
						"source_path": sourcePath,
					})
				}
			}
		}
		return nil
	}

	if content, ok := manifestData["content"].(map[string]any); ok {
		if addFiles, ok := content["add_files"].([]any); ok {
			if err := processAddFiles(addFiles); err != nil {
				return nil, err
			}
		}
	}

	if qm, ok := manifestData["qm"].(map[string]any); ok {
		if qmContent, ok := qm["content"].(map[string]any); ok {
			if addFiles, ok := qmContent["add_files"].([]any); ok {
				if err := processAddFiles(addFiles); err != nil {
					return nil, err
				}
			}
		}
	}

	return localFiles, nil
}

func downloadArtifactViaAPI(ctx context.Context, baseURL, name, outDir string) error {
	if strings.TrimSpace(outDir) == "" {
		outDir = "./output"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	base := strings.TrimRight(baseURL, "/")
	urlStr := base + "/v1/builds/" + url.PathEscape(name) + "/artifact"

	deadline := time.Now().Add(30 * time.Minute)

	httpClient := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 2 * time.Minute,
			IdleConnTimeout:       5 * time.Minute,
		},
	}

	warned := false
	for {
		if ctx.Err() != nil || time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for artifact to become ready")
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
		if strings.TrimSpace(authToken) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(authToken))
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			filename := name + ".artifact"
			contentType := resp.Header.Get("Content-Type")
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				if i := strings.Index(cd, "filename="); i >= 0 {
					f := strings.Trim(cd[i+9:], "\" ")
					if f != "" {
						filename = f
					}
				}
			}
			if at := strings.TrimSpace(resp.Header.Get("X-AIB-Artifact-Type")); at != "" {
				fmt.Printf("Artifact type: %s\n", at)
			}
			if comp := strings.TrimSpace(resp.Header.Get("X-AIB-Compression")); comp != "" {
				fmt.Printf("Compression: %s\n", comp)
			}
			if root := strings.TrimSpace(resp.Header.Get("X-AIB-Archive-Root")); root != "" {
				fmt.Printf("Archive root: %s\n", root)
			}
			outPath := filepath.Join(outDir, filename)
			tmp := outPath + ".partial"
			f, err := os.Create(tmp)
			if err != nil {
				resp.Body.Close()
				return err
			}
			if cl := strings.TrimSpace(resp.Header.Get("Content-Length")); cl != "" {
				// Known size: nice progress bar
				// Convert to int64
				var total int64
				fmt.Sscan(cl, &total)
				bar := progressbar.NewOptions64(
					total,
					progressbar.OptionSetDescription("Downloading"),
					progressbar.OptionShowBytes(true),
					progressbar.OptionSetWidth(15),
					progressbar.OptionThrottle(65*time.Millisecond),
					progressbar.OptionShowCount(),
					progressbar.OptionClearOnFinish(),
				)
				reader := io.TeeReader(resp.Body, bar)
				if _, copyErr := io.Copy(f, reader); copyErr != nil {
					f.Close()
					os.Remove(tmp)
					return copyErr
				}
				_ = bar.Finish()
				fmt.Println()
			} else {
				bar := progressbar.NewOptions(
					-1,
					progressbar.OptionSetDescription("Downloading"),
					progressbar.OptionSpinnerType(14),
					progressbar.OptionClearOnFinish(),
				)
				reader := io.TeeReader(resp.Body, bar)
				if _, copyErr := io.Copy(f, reader); copyErr != nil {
					f.Close()
					os.Remove(tmp)
					return copyErr
				}
				_ = bar.Finish()
				fmt.Println()
			}
			resp.Body.Close()
			f.Close()
			if err := os.Rename(tmp, outPath); err != nil {
				return err
			}
			fmt.Printf("Artifact downloaded to %s\n", outPath)

			// If the artifact is a tar archive (directory export), optionally extract it
			if strings.HasPrefix(contentType, "application/x-tar") || strings.HasPrefix(contentType, "application/gzip") || strings.HasSuffix(strings.ToLower(outPath), ".tar") || strings.HasSuffix(strings.ToLower(outPath), ".tar.gz") {
				if !compressArtifacts {
					destDir := strings.TrimSuffix(outPath, ".tar")
					destDir = strings.TrimSuffix(destDir, ".gz")
					if err := os.MkdirAll(destDir, 0o755); err != nil {
						return fmt.Errorf("create extract dir: %w", err)
					}
					if err := extractTar(outPath, destDir); err != nil {
						return fmt.Errorf("extract tar: %w", err)
					}
					fmt.Printf("Extracted to %s\n", destDir)
				}
			}
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		msg := strings.ToLower(strings.TrimSpace(string(body)))
		resp.Body.Close()
		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusConflict || strings.Contains(msg, "not ready") {
			if !warned {
				fmt.Println("Artifact not ready yet. Waiting...")
				warned = true
			}
			time.Sleep(3 * time.Second)
			continue
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func extractTar(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(tarPath), ".gz") {
		gr, gzErr := gzip.NewReader(f)
		if gzErr == nil {
			defer gr.Close()
			r = gr
		}
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil && !os.IsExist(err) {
				return err
			}
		default:
			// ignore other types
		}
	}
	return nil
}

func runDownload(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	if strings.TrimSpace(serverURL) == "" {
		fmt.Println("Error: --server is required (or set CAIB_SERVER)")
		os.Exit(1)
	}

	if strings.TrimSpace(authToken) == "" {
		if tok, err := loadTokenFromKubeconfig(); err == nil && strings.TrimSpace(tok) != "" {
			authToken = tok
		}
	}
	var opts []buildapiclient.Option
	if strings.TrimSpace(authToken) != "" {
		opts = append(opts, buildapiclient.WithAuthToken(strings.TrimSpace(authToken)))
	}
	api, err := buildapiclient.New(serverURL, opts...)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	st, err := api.GetBuild(ctx, buildName)
	if err != nil {
		fmt.Printf("Error getting build %s: %v\n", buildName, err)
		os.Exit(1)
	}
	if st.Phase != "Completed" {
		fmt.Printf("Build %s is not completed (status: %s). Cannot download artifacts.\n", buildName, st.Phase)
		os.Exit(1)
	}

	if err := downloadArtifactViaAPI(ctx, serverURL, buildName, outputDir); err != nil {
		fmt.Printf("Download failed: %v\n", err)
		os.Exit(1)
	}
}

func runList(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	if strings.TrimSpace(serverURL) == "" {
		fmt.Println("Error: --server is required (or set CAIB_SERVER)")
		os.Exit(1)
	}
	if strings.TrimSpace(authToken) == "" {
		if tok, err := loadTokenFromKubeconfig(); err == nil && strings.TrimSpace(tok) != "" {
			authToken = tok
		}
	}
	var opts []buildapiclient.Option
	if strings.TrimSpace(authToken) != "" {
		opts = append(opts, buildapiclient.WithAuthToken(strings.TrimSpace(authToken)))
	}
	api, err := buildapiclient.New(serverURL, opts...)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	items, err := api.ListBuilds(ctx)
	if err != nil {
		fmt.Printf("Error listing ImageBuilds: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Println("No ImageBuilds found")
		return
	}
	fmt.Printf("%-20s %-12s %-20s %-20s %-20s\n", "NAME", "STATUS", "MESSAGE", "CREATED", "ARTIFACT")
	for _, it := range items {
		fmt.Printf("%-20s %-12s %-20s %-20s %-20s\n", it.Name, it.Phase, it.Message, it.CreatedAt, "")
	}
}

func loadTokenFromKubeconfig() (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// First, ask client-go to build a client config. This will execute any exec credential plugins
	// (e.g., OpenShift login) and populate a usable BearerToken.
	deferred := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	if restCfg, err := deferred.ClientConfig(); err == nil && restCfg != nil {
		if t := strings.TrimSpace(restCfg.BearerToken); t != "" {
			return t, nil
		}
		if f := strings.TrimSpace(restCfg.BearerTokenFile); f != "" {
			if b, rerr := os.ReadFile(f); rerr == nil {
				if t := strings.TrimSpace(string(b)); t != "" {
					return t, nil
				}
			}
		}
	}

	// Fallback to parsing raw kubeconfig for legacy token fields
	rawCfg, err := loadingRules.Load()
	if err != nil || rawCfg == nil {
		return "", fmt.Errorf("cannot load kubeconfig: %w", err)
	}
	ctxName := rawCfg.CurrentContext
	if strings.TrimSpace(ctxName) == "" {
		return "", fmt.Errorf("no current kube context")
	}
	ctx := rawCfg.Contexts[ctxName]
	if ctx == nil {
		return "", fmt.Errorf("missing context %s", ctxName)
	}
	ai := rawCfg.AuthInfos[ctx.AuthInfo]
	if ai == nil {
		return "", fmt.Errorf("missing auth info for context %s", ctxName)
	}
	if strings.TrimSpace(ai.Token) != "" {
		return strings.TrimSpace(ai.Token), nil
	}
	if ai.AuthProvider != nil && ai.AuthProvider.Config != nil {
		if t := strings.TrimSpace(ai.AuthProvider.Config["access-token"]); t != "" {
			return t, nil
		}
		if t := strings.TrimSpace(ai.AuthProvider.Config["id-token"]); t != "" {
			return t, nil
		}
		if t := strings.TrimSpace(ai.AuthProvider.Config["token"]); t != "" {
			return t, nil
		}
	}
	if path, err := exec.LookPath("oc"); err == nil && path != "" {
		out, err := exec.Command(path, "whoami", "-t").Output()
		if err == nil {
			if t := strings.TrimSpace(string(out)); t != "" {
				return t, nil
			}
		}
	}
	return "", fmt.Errorf("no bearer token found in kubeconfig")
}
