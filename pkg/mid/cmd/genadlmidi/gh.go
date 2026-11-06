package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const (
	owner   = "Wohlstand"
	repo    = "libADLMIDI"
	destDir = "./fm_banks_new" // local destination
	apiBase = "https://api.github.com"
)

func DownloadFMBanks() {
	if err := downloadINI(); err != nil {
		fmt.Printf("download banks.ini: %v\n", err)
		return
	}

	var files []fileJob

	rootPath := "fm_banks_new" // folder in the repo

	// Map repo path "fm_banks/foo/bar.bin" -> "./fm_banks/foo/bar.bin".
	err := walk(owner, repo, rootPath, func(c GitHubContent) {
		rel := strings.TrimPrefix(c.Path, rootPath)
		rel = strings.TrimPrefix(rel, "/")

		dst := filepath.Join(destDir, filepath.FromSlash(rel))
		files = append(files, fileJob{dst: dst, url: c.DownloadURL})
	})

	if err != nil {
		fmt.Printf("walk: %v\n", err)
		return
	}

	fmt.Printf("found %d files\n", len(files))

	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.dst), 0o755); err != nil {
			fmt.Printf("mkdir %s: %v\n", filepath.Dir(f.dst), err)
			return
		}
	}

	const workers = 8

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed int

	sem := make(chan struct{}, workers)

	for _, f := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			err := downloadFile(f.url, f.dst)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				fmt.Printf("download %s: %v\n", f.dst, err)
				return
			}

			fmt.Printf("ok %s\n", f.dst)
		})
	}

	wg.Wait()

	if failed > 0 {
		fmt.Printf("done with %d failures\n", failed)
		os.Exit(1)
	}
	fmt.Println("done.")
}

type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`         // full repo-relative path
	Type        string `json:"type"`         // "file" or "dir"
	DownloadURL string `json:"download_url"` // null for dirs
}

type fileJob struct {
	dst, url string
}

func walk(owner, repo, path string, fn func(GitHubContent)) error {
	contents, err := listContents(owner, repo, path)
	if err != nil {
		return err
	}
	for _, c := range contents {
		switch c.Type {
		case "file":
			fn(c)
		case "dir":
			if err := walk(owner, repo, c.Path, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// listContents calls the GitHub Contents API for a given repo path.
func listContents(owner, repo, repoPath string) ([]GitHubContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		apiBase, owner, repo, path.Clean(repoPath),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: %s: %s", url, resp.Status, body)
	}

	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadINI() error {
	contents, err := listContents(owner, repo, "")
	if err != nil {
		return fmt.Errorf("list repository root: %w", err)
	}

	for _, c := range contents {
		if c.Type != "file" || c.Name != "banks.ini" {
			continue
		}

		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("create destination directory %s: %w", destDir, err)
		}

		dst := filepath.Join(destDir, c.Name)
		if err := downloadFile(c.DownloadURL, dst); err != nil {
			return fmt.Errorf("download %s: %w", dst, err)
		}
		fmt.Printf("ok %s\n", dst)
		return nil
	}

	return fmt.Errorf("banks.ini not found in repository root")
}
