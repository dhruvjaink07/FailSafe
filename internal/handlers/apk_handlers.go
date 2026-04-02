package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type uploadedAPK struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Package      string    `json:"package"`
	Activity     string    `json:"activity"`
	OriginalName string    `json:"original_name"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

var (
	uploadedAPKMu sync.RWMutex
	uploadedAPKs  = map[string]uploadedAPK{}
)

func UploadAPKHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(1024 << 20); err != nil {
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			file, header, err = r.FormFile("apk")
			if err != nil {
				http.Error(w, "missing apk file field (use file or apk)", http.StatusBadRequest)
				return
			}
		}
		defer file.Close()

		id := uuid.NewString()
		baseDir, err := resolveAPKUploadDir()
		if err != nil {
			http.Error(w, "failed to resolve upload directory", http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			http.Error(w, "failed to prepare upload directory", http.StatusInternalServerError)
			return
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == "" {
			ext = ".apk"
		}
		dstPath := filepath.Join(baseDir, id+ext)

		dst, err := os.Create(dstPath)
		if err != nil {
			http.Error(w, "failed to save uploaded apk", http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(dst, file); err != nil {
			_ = dst.Close()
			_ = os.Remove(dstPath)
			http.Error(w, "failed to write uploaded apk", http.StatusInternalServerError)
			return
		}
		_ = dst.Close()

		pkg, activity, err := extractAPKMetadata(dstPath)
		if err != nil {
			_ = os.Remove(dstPath)
			http.Error(w, "failed to extract apk metadata: "+err.Error(), http.StatusBadRequest)
			return
		}

		record := uploadedAPK{
			ID:           id,
			Path:         dstPath,
			Package:      pkg,
			Activity:     activity,
			OriginalName: header.Filename,
			UploadedAt:   time.Now(),
		}

		setUploadedAPK(record)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       id,
			"apk":      id,
			"path":     dstPath,
			"package":  pkg,
			"activity": activity,
		})
	}
}

func resolveAPKUploadDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("APK_UPLOAD_DIR")); configured != "" {
		if filepath.IsAbs(configured) {
			return configured, nil
		}
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, configured), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "uploads", "apks"), nil
}

func setUploadedAPK(apk uploadedAPK) {
	uploadedAPKMu.Lock()
	defer uploadedAPKMu.Unlock()
	uploadedAPKs[apk.ID] = apk
}

func getUploadedAPK(id string) (uploadedAPK, bool) {
	uploadedAPKMu.RLock()
	defer uploadedAPKMu.RUnlock()
	apk, ok := uploadedAPKs[strings.TrimSpace(id)]
	return apk, ok
}

func extractAPKMetadata(apkPath string) (string, string, error) {
	out, err := runAAPT(apkPath)
	if err != nil {
		return "", "", err
	}

	pkgRe := regexp.MustCompile(`(?m)^package:\s+name=['"]([^'"]+)['"]`)
	activityRe := regexp.MustCompile(`(?m)^launchable-activity:\s+name=['"]([^'"]+)['"]`)

	pkgMatch := pkgRe.FindStringSubmatch(out)
	if len(pkgMatch) < 2 {
		return "", "", fmt.Errorf("package name not found in aapt output: %s", compactAAPTOutput(out))
	}

	activityMatch := activityRe.FindStringSubmatch(out)
	if len(activityMatch) < 2 {
		return "", "", fmt.Errorf("launchable activity not found in aapt output: %s", compactAAPTOutput(out))
	}

	return strings.TrimSpace(pkgMatch[1]), strings.TrimSpace(activityMatch[1]), nil
}

func runAAPT(apkPath string) (string, error) {
	candidates := make([]string, 0, 4)

	if p := strings.TrimSpace(os.Getenv("AAPT_PATH")); p != "" {
		candidates = append(candidates, p)
	}

	if sdk := firstNonEmpty(strings.TrimSpace(os.Getenv("ANDROID_SDK_ROOT")), strings.TrimSpace(os.Getenv("ANDROID_HOME"))); sdk != "" {
		patterns := []string{
			filepath.Join(sdk, "build-tools", "*", "aapt*.exe"),
			filepath.Join(sdk, "build-tools", "*", "aapt*"),
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			sort.Strings(matches)
			for i := len(matches) - 1; i >= 0; i-- {
				candidates = append(candidates, matches[i])
			}
		}
	}

	if runtime.GOOS == "windows" {
		candidates = append(candidates, "aapt.exe")
	}
	candidates = append(candidates, "aapt")

	var lastErr error
	for _, bin := range candidates {
		cmd := exec.Command(bin, "dump", "badging", apkPath)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
		lastErr = fmt.Errorf("%s: %w", bin, err)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("aapt command unavailable")
	}
	return "", lastErr
}

func compactAAPTOutput(out string) string {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return "<empty output>"
	}
	const maxLen = 600
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen] + "..."
}
