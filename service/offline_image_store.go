package service

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultOfflineImageDir = "offline-images"
	// MaxOfflineReferenceBytes bounds the reference image a user may attach to an
	// offline edit task.
	MaxOfflineReferenceBytes = 10 << 20
)

// offlineImageNamePattern is the only name shape the content endpoint opens, so
// a name read from the database can never escape its task directory.
var offlineImageNamePattern = regexp.MustCompile(`^[a-z]+-\d+-[A-Za-z0-9]{4,16}\.[a-z0-9]{1,8}$`)

// OfflineImageRootDir is where task files live. The default resolves inside the
// process working directory, which is the mounted data volume in production.
func OfflineImageRootDir() string {
	return common.GetEnvOrDefaultString("OFFLINE_IMAGE_DIR", defaultOfflineImageDir)
}

func OfflineImageTaskDir(userId int, taskId int64) string {
	return filepath.Join(OfflineImageRootDir(), strconv.Itoa(userId), strconv.FormatInt(taskId, 10))
}

func offlineImageExt(mime string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0])) {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/avif":
		return "avif"
	default:
		return "png"
	}
}

func normalizeImageMime(mime string) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	if !strings.HasPrefix(normalized, "image/") {
		return "image/png"
	}
	return normalized
}

// StoreOfflineImageFile writes one image into the task directory. kind keeps the
// reference apart from the outputs so cleanup and the UI can tell them apart.
func StoreOfflineImageFile(userId int, taskId int64, kind string, index int, mime string, data []byte) (model.OfflineImageFile, error) {
	if kind != "ref" && kind != "out" {
		return model.OfflineImageFile{}, errors.New("invalid offline image kind")
	}
	if len(data) == 0 {
		return model.OfflineImageFile{}, errors.New("empty offline image payload")
	}
	dir := OfflineImageTaskDir(userId, taskId)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.OfflineImageFile{}, err
	}
	normalized := normalizeImageMime(mime)
	name := kind + "-" + strconv.Itoa(index) + "-" + common.GetRandomString(8) + "." + offlineImageExt(normalized)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return model.OfflineImageFile{}, err
	}
	return model.OfflineImageFile{Name: name, Mime: normalized}, nil
}

func ReadOfflineImageFile(userId int, taskId int64, name string) ([]byte, error) {
	if !offlineImageNamePattern.MatchString(name) {
		return nil, errors.New("invalid offline image name")
	}
	return os.ReadFile(filepath.Join(OfflineImageTaskDir(userId, taskId), name))
}

func RemoveOfflineImageTaskDir(userId int, taskId int64) error {
	return os.RemoveAll(OfflineImageTaskDir(userId, taskId))
}
